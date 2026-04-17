package smoothcomp

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/kmicac/smoothcomp-scraper/internal/core/contract"
	coreerrors "github.com/kmicac/smoothcomp-scraper/internal/core/errors"
)

type embeddedEvent struct {
	ID                   int    `json:"id"`
	Title                string `json:"title"`
	CoverImage           string `json:"cover_image"`
	CoverImageFallback   string `json:"cover_image_fallback"`
	URL                  string `json:"url"`
	DaysToStart          *int   `json:"days_to_start"`
	EventPeriod          string `json:"eventPeriod"`
	LocationCountry      string `json:"location_country"`
	LocationCountryHuman string `json:"location_country_human"`
	LocationCity         string `json:"location_city"`
	StartDate            string `json:"startdate"`
	EndDate              string `json:"enddate"`
}

func parseEventCatalogHTML(baseURL, eventType string, body []byte, snapshotID string) ([]contract.Event, error) {
	if events, err := parseEventsFromScript(body, eventType, snapshotID); err == nil && len(events) > 0 {
		return events, nil
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.parse_event_catalog_html", true, "failed to parse event catalog html", err)
	}

	var events []contract.Event
	doc.Find(".event-card").Each(func(_ int, card *goquery.Selection) {
		titleLink := card.Find("a.event-title").First()
		name := strings.TrimSpace(titleLink.Text())
		eventURL, _ := titleLink.Attr("href")
		if eventURL == "" {
			eventURL, _ = card.Find("a.image-container").First().Attr("href")
		}
		sourceURL := normalizeEventURL(baseURL, eventURL)
		if name == "" || sourceURL == "" {
			return
		}

		city, country := extractEventLocation(card)
		item := contract.Event{
			SourceID:        extractIDFromURL(sourceURL),
			SourceURL:       sourceURL,
			Name:            name,
			City:            city,
			Country:         country,
			CountryCode:     extractEventCountryCode(card),
			Status:          statusFromEventType(eventType),
			RawReferenceIDs: []string{snapshotID},
			Attributes: map[string]string{
				"date_text": strings.TrimSpace(card.Find(".date").First().Text()),
				"days_text": strings.TrimSpace(card.Find(".days").First().Text()),
			},
		}
		events = append(events, item)
	})

	return events, nil
}

func parseEventsFromScript(body []byte, eventType, snapshotID string) ([]contract.Event, error) {
	arrayBytes, err := extractEventsArray(body)
	if err != nil {
		return nil, err
	}

	var payload []embeddedEvent
	if err := json.Unmarshal(arrayBytes, &payload); err != nil {
		return nil, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.parse_event_catalog_script", true, "failed to decode embedded event payload", err)
	}

	items := make([]contract.Event, 0, len(payload))
	for _, event := range payload {
		item := contract.Event{
			SourceID:        strconv.Itoa(event.ID),
			SourceURL:       strings.TrimSpace(event.URL),
			Name:            strings.TrimSpace(event.Title),
			City:            strings.TrimSpace(event.LocationCity),
			Country:         strings.TrimSpace(event.LocationCountryHuman),
			CountryCode:     strings.ToUpper(strings.TrimSpace(event.LocationCountry)),
			Status:          statusFromEventType(eventType),
			StartsAt:        strings.TrimSpace(event.StartDate),
			EndsAt:          strings.TrimSpace(event.EndDate),
			RawReferenceIDs: []string{snapshotID},
			Attributes: map[string]string{
				"date_text": strings.TrimSpace(event.EventPeriod),
			},
		}
		if event.DaysToStart != nil {
			item.Attributes["days_text"] = strconv.Itoa(*event.DaysToStart)
		}
		items = append(items, item)
	}

	return items, nil
}

func extractEventsArray(body []byte) ([]byte, error) {
	start := bytes.Index(body, []byte("var events"))
	if start < 0 {
		return nil, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.extract_events_array", true, "embedded events not found", nil)
	}
	open := bytes.IndexByte(body[start:], '[')
	if open < 0 {
		return nil, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.extract_events_array", true, "embedded events array start not found", nil)
	}
	open += start

	depth := 0
	inString := false
	escape := false
	end := -1
	for i := open; i < len(body); i++ {
		ch := body[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			if ch == '\\' {
				escape = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			continue
		}
		if ch == '[' {
			depth++
		}
		if ch == ']' {
			depth--
			if depth == 0 {
				end = i
				break
			}
		}
	}
	if end == -1 {
		return nil, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.extract_events_array", true, "embedded events array end not found", nil)
	}
	return body[open : end+1], nil
}

func normalizeEventURL(baseURL string, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	base := strings.TrimRight(baseURL, "/")
	if strings.HasPrefix(href, "/") {
		return base + href
	}
	return base + "/" + href
}

func extractEventCountryCode(card *goquery.Selection) string {
	classAttr, _ := card.Find(".flag-icon").First().Attr("class")
	re := regexp.MustCompile(`flag-icon-([a-z]{2})`)
	match := re.FindStringSubmatch(classAttr)
	if len(match) < 2 {
		return ""
	}
	return strings.ToUpper(match[1])
}

func extractEventLocation(card *goquery.Selection) (string, string) {
	parts := make([]string, 0, 4)
	card.Find(".location span").Each(func(_ int, span *goquery.Selection) {
		text := strings.TrimSpace(span.Text())
		text = strings.Trim(text, ",")
		if text == "" || text == "," {
			return
		}
		parts = append(parts, text)
	})
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return "", parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", "), parts[len(parts)-1]
}

func extractIDFromURL(rawURL string) string {
	parts := strings.Split(strings.TrimRight(rawURL, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func statusFromEventType(eventType string) string {
	if eventType == "past" {
		return "completed"
	}
	return "scheduled"
}

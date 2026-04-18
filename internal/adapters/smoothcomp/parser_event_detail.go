package smoothcomp

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/kmicac/smoothcomp-scraper/internal/core/contract"
	coreerrors "github.com/kmicac/smoothcomp-scraper/internal/core/errors"
)

type eventDetailParsed struct {
	Event contract.Event
}

type eventPanelsData struct {
	VenueName     string
	City          string
	Country       string
	VenueAddress  string
	OrganizerName string
	Attributes    map[string]string
}

type eventCMSData struct {
	Attributes map[string]string
}

type eventJSONLDDocument struct {
	Type        string `json:"@type"`
	Name        string `json:"name"`
	StartDate   string `json:"startDate"`
	EndDate     string `json:"endDate"`
	Image       string `json:"image"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Location    struct {
		Name    string `json:"name"`
		Address struct {
			AddressLocality string `json:"addressLocality"`
			AddressCountry  string `json:"addressCountry"`
			Description     string `json:"description"`
		} `json:"address"`
	} `json:"location"`
	Organizer struct {
		Name string `json:"name"`
	} `json:"organizer"`
}

func parseEventDetailHTML(body []byte, fallbackEventID, fallbackEventURL, snapshotID string) (eventDetailParsed, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return eventDetailParsed{}, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.parse_event_detail_html", true, "failed to parse event detail html", err)
	}

	event := contract.Event{
		SourceID:        fallbackEventID,
		SourceURL:       fallbackEventURL,
		RawReferenceIDs: []string{snapshotID},
		Attributes:      map[string]string{},
	}

	if ld := parseEventJSONLDDocument(doc); ld != nil {
		event.Name = strings.TrimSpace(ld.Name)
		event.Description = strings.TrimSpace(ld.Description)
		event.ImageURL = strings.TrimSpace(ld.Image)
		event.StartsAt = strings.TrimSpace(ld.StartDate)
		event.EndsAt = strings.TrimSpace(ld.EndDate)
		event.VenueName = strings.TrimSpace(ld.Location.Name)
		event.City = strings.TrimSpace(ld.Location.Address.AddressLocality)
		event.Country = strings.TrimSpace(ld.Location.Address.AddressCountry)
		event.VenueAddress = strings.TrimSpace(ld.Location.Address.Description)
		event.OrganizerName = strings.TrimSpace(ld.Organizer.Name)
		if event.SourceURL == "" {
			event.SourceURL = strings.TrimSpace(ld.URL)
		}
		if event.SourceID == "" && event.SourceURL != "" {
			event.SourceID = extractIDFromURL(event.SourceURL)
		}
	}

	if event.Name == "" {
		event.Name = firstNonEmpty(
			strings.TrimSpace(doc.Find("h1").First().Text()),
			strings.TrimSpace(doc.Find(".event-name").First().Text()),
			strings.TrimSpace(doc.Find(".page-title").First().Text()),
		)
	}
	if event.Description == "" {
		event.Description = extractVisibleEventDescription(doc)
	}
	if event.ImageURL == "" {
		event.ImageURL = firstNonEmpty(
			strings.TrimSpace(attrOrEmpty(doc.Find("meta[property='og:image']").First(), "content")),
			strings.TrimSpace(attrOrEmpty(doc.Find(".event-cover img").First(), "src")),
		)
	}
	if event.SourceID == "" && event.SourceURL != "" {
		event.SourceID = extractIDFromURL(event.SourceURL)
	}
	if event.SourceID == "" {
		return eventDetailParsed{}, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.parse_event_detail_html", true, "event detail did not contain a resolvable event id", nil)
	}

	return eventDetailParsed{Event: event}, nil
}

func parseEventInfoPanelsJSON(body []byte) (eventPanelsData, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return eventPanelsData{}, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.parse_event_info_panels", true, "failed to decode event info panels json", err)
	}

	attributes := map[string]string{}
	for key, value := range payload {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				attributes[key] = strings.TrimSpace(typed)
			}
		case float64:
			attributes[key] = strconv.FormatFloat(typed, 'f', -1, 64)
		case bool:
			if typed {
				attributes[key] = "true"
			} else {
				attributes[key] = "false"
			}
		}
	}

	data := eventPanelsData{
		VenueName:     firstNonEmpty(stringValue(payload["location_name"]), stringValue(payload["venue_name"])),
		City:          firstNonEmpty(stringValue(payload["location_city"]), stringValue(payload["city"])),
		Country:       firstNonEmpty(stringValue(payload["location_country_human"]), stringValue(payload["location_country"]), stringValue(payload["country"])),
		VenueAddress:  firstNonEmpty(stringValue(payload["location_address"]), stringValue(payload["address"])),
		OrganizerName: nestedString(payload, "organizer", "name"),
		Attributes:    attributes,
	}
	return data, nil
}

func parseEventCMSJSON(body []byte) (eventCMSData, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return eventCMSData{}, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.parse_event_cms", true, "failed to decode event cms json", err)
	}

	attributes := map[string]string{}
	if blocks, ok := payload["infoPageBlocks"].([]any); ok {
		attributes["cms_blocks_count"] = strconv.Itoa(len(blocks))
		for index, block := range blocks {
			if title := nestedString(block, "title"); title != "" {
				attributes["cms_block_"+strconv.Itoa(index)+"_title"] = title
			}
			if kind := nestedString(block, "type"); kind != "" {
				attributes["cms_block_"+strconv.Itoa(index)+"_type"] = kind
			}
		}
	}
	return eventCMSData{Attributes: attributes}, nil
}

func parseEventJSONLDDocument(doc *goquery.Document) *eventJSONLDDocument {
	var result *eventJSONLDDocument

	doc.Find("script[type='application/ld+json']").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		raw := strings.TrimSpace(selection.Text())
		if raw == "" {
			return true
		}

		var single eventJSONLDDocument
		if err := json.Unmarshal([]byte(raw), &single); err == nil && isSportsEvent(single.Type) {
			result = &single
			return false
		}

		var list []eventJSONLDDocument
		if err := json.Unmarshal([]byte(raw), &list); err == nil {
			for i := range list {
				if isSportsEvent(list[i].Type) {
					result = &list[i]
					return false
				}
			}
		}
		return true
	})

	return result
}

func isSportsEvent(eventType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(eventType))
	return normalized == "sportsevent" || normalized == "event"
}

func extractVisibleEventDescription(doc *goquery.Document) string {
	for _, selector := range []string{
		".event-description",
		".event-description-text",
		".event-page-description",
		".event-info__description",
		"#event-description",
		"[data-testid='event-description']",
		".event-body .description",
		".event-content .description",
		".event-info .description",
	} {
		if text := normalizeVisibleText(doc.Find(selector).First().Text()); text != "" {
			return text
		}
	}

	description := ""
	doc.Find("div, section, article, p").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		attrs := strings.ToLower(strings.TrimSpace(selection.AttrOr("class", "") + " " + selection.AttrOr("id", "")))
		parentAttrs := strings.ToLower(strings.TrimSpace(selection.Parent().AttrOr("class", "") + " " + selection.Parent().AttrOr("id", "")))
		if !strings.Contains(attrs, "description") {
			return true
		}
		if !strings.Contains(attrs, "event") && !strings.Contains(parentAttrs, "event") {
			return true
		}
		text := normalizeVisibleText(selection.Text())
		if text == "" {
			return true
		}
		description = text
		return false
	})

	return description
}

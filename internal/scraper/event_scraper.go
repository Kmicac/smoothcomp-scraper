package scraper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/kmicac/smoothcomp-scraper/internal/config"
	"github.com/kmicac/smoothcomp-scraper/internal/models"
	"github.com/kmicac/smoothcomp-scraper/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ScrapeEvents fetches and stores events for the given type and country.
func (s *Scraper) ScrapeEvents(eventType string, countryCode string) error {
	job := s.createJob("events_" + eventType)

	events, err := s.ScrapeEventsByCountry(eventType, countryCode)
	if err != nil {
		s.failJob(job, err)
		return err
	}

	savedCount := 0
	for i := range events {
		if err := s.SaveEvent(&events[i]); err != nil {
			logger.Error("Failed to save event",
				zap.String("event", events[i].Name),
				zap.Error(err))
			continue
		}
		savedCount++
	}

	job.ItemsScraped = savedCount
	s.completeJob(job)

	logger.Info("Event scraping completed",
		zap.String("type", eventType),
		zap.Int("saved", savedCount),
		zap.Int("total", len(events)))

	return nil
}

// ScrapeEventsByCountry scrapes events from SmoothComp listings.
func (s *Scraper) ScrapeEventsByCountry(eventType string, countryCode string) ([]models.Event, error) {
	eventsURL, err := s.buildEventsURL(eventType, countryCode)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest("GET", eventsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating events request: %w", err)
	}

	req.Header.Set("User-Agent", s.config.Scraper.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("events endpoint returned status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading events response: %w", err)
	}

	if events, parseErr := parseEventsFromScript(bodyBytes, eventType); parseErr == nil && len(events) > 0 {
		return events, nil
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("error parsing events HTML: %w", err)
	}

	var events []models.Event
	doc.Find(".event-card").Each(func(_ int, card *goquery.Selection) {
		event := models.Event{
			EventType: eventType,
			ScrapedAt: time.Now(),
		}

		section := strings.TrimSpace(card.ParentsFiltered(".margin-bottom-xs-64").First().Find("h2").First().Text())
		if section != "" {
			event.Section = section
		}

		titleLink := card.Find("a.event-title").First()
		event.Name = strings.TrimSpace(titleLink.Text())

		eventURL, _ := titleLink.Attr("href")
		if eventURL == "" {
			eventURL, _ = card.Find("a.image-container").First().Attr("href")
		}
		event.EventURL = normalizeEventURL(s.config.Scraper.BaseURL, eventURL)
		event.ExternalID = ExtractIDFromURL(event.EventURL)

		imageURL, _ := card.Find("img").First().Attr("src")
		event.ImageURL = strings.TrimSpace(imageURL)

		event.CountryCode = extractEventCountryCode(card)
		if event.CountryCode == "" {
			event.CountryCode = strings.ToUpper(strings.TrimSpace(countryCode))
		}

		event.City, event.Country = extractEventLocation(card)

		event.DateText = strings.TrimSpace(card.Find(".date").First().Text())
		event.DaysText = strings.TrimSpace(card.Find(".days").First().Text())

		if event.EventURL != "" && event.Name != "" {
			events = append(events, event)
		}
	})

	return events, nil
}

func (s *Scraper) buildEventsURL(eventType string, countryCode string) (string, error) {
	if eventType != "past" && eventType != "upcoming" {
		return "", fmt.Errorf("invalid event type: %s", eventType)
	}

	base := strings.TrimRight(s.config.Scraper.BaseURL, "/")
	rawURL := fmt.Sprintf("%s/en/events/%s", base, eventType)
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid events URL: %w", err)
	}

	query := parsed.Query()
	if countryCode != "" {
		query.Set("countries", strings.ToUpper(countryCode))
	}
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}

// SaveEvent creates or updates an event in the database.
func (s *Scraper) SaveEvent(event *models.Event) error {
	db := config.GetDB()
	var existing models.Event

	query := db.Where("event_url = ?", event.EventURL)
	if event.EventURL == "" && event.ExternalID != "" {
		query = db.Where("external_id = ?", event.ExternalID)
	}

	result := query.First(&existing)
	if result.Error == nil {
		event.ID = existing.ID
		event.CreatedAt = existing.CreatedAt
		if err := db.Save(event).Error; err != nil {
			return fmt.Errorf("failed to update event: %w", err)
		}
		return nil
	}

	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check event: %w", result.Error)
	}

	if err := db.Create(event).Error; err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}

	return nil
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

	city := strings.Join(parts[:len(parts)-1], ", ")
	country := parts[len(parts)-1]
	return city, country
}

type embeddedEvent struct {
	ID                   int    `json:"id"`
	Title                string `json:"title"`
	CoverImage           string `json:"cover_image"`
	CoverImageFallback   string `json:"cover_image_fallback"`
	URL                  string `json:"url"`
	DaysToStart          *int   `json:"days_to_start"`
	EventPeriod          string `json:"eventPeriod"`
	EventEnded           bool   `json:"eventEnded"`
	LocationCountry      string `json:"location_country"`
	LocationCountryHuman string `json:"location_country_human"`
	LocationCity         string `json:"location_city"`
	StartDate            string `json:"startdate"`
	EndDate              string `json:"enddate"`
}

func parseEventsFromScript(body []byte, eventType string) ([]models.Event, error) {
	arrayBytes, err := extractEventsArray(body)
	if err != nil {
		return nil, err
	}

	var payload []embeddedEvent
	if err := json.Unmarshal(arrayBytes, &payload); err != nil {
		return nil, fmt.Errorf("error decoding embedded events: %w", err)
	}

	events := make([]models.Event, 0, len(payload))
	for _, item := range payload {
		event := models.Event{
			ExternalID:  strconv.Itoa(item.ID),
			Name:        strings.TrimSpace(item.Title),
			EventURL:    strings.TrimSpace(item.URL),
			ImageURL:    strings.TrimSpace(item.CoverImage),
			City:        strings.TrimSpace(item.LocationCity),
			Country:     strings.TrimSpace(item.LocationCountryHuman),
			CountryCode: strings.ToUpper(strings.TrimSpace(item.LocationCountry)),
			DateText:    strings.TrimSpace(item.EventPeriod),
			EventType:   eventType,
			ScrapedAt:   time.Now(),
		}

		if event.ImageURL == "" {
			event.ImageURL = strings.TrimSpace(item.CoverImageFallback)
		}

		if item.DaysToStart != nil {
			if *item.DaysToStart >= 0 {
				event.DaysText = fmt.Sprintf("%d days left", *item.DaysToStart)
			} else {
				event.DaysText = fmt.Sprintf("%d days ago", -(*item.DaysToStart))
			}
		}

		if event.EventURL != "" && event.Name != "" {
			events = append(events, event)
		}
	}

	return events, nil
}

func extractEventsArray(body []byte) ([]byte, error) {
	start := bytes.Index(body, []byte("var events"))
	if start < 0 {
		return nil, fmt.Errorf("embedded events not found")
	}

	open := bytes.IndexByte(body[start:], '[')
	if open < 0 {
		return nil, fmt.Errorf("embedded events array start not found")
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
		return nil, fmt.Errorf("embedded events array end not found")
	}

	return body[open : end+1], nil
}

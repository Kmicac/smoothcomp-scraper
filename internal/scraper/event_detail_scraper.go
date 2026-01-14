package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/kmicac/smoothcomp-scraper/internal/config"
	"github.com/kmicac/smoothcomp-scraper/internal/models"
	"gorm.io/gorm"
)

type EventDetails struct {
	EventID         string                 `json:"event_id"`
	EventURL        string                 `json:"event_url"`
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	StartDate       string                 `json:"start_date"`
	EndDate         string                 `json:"end_date"`
	ImageURL        string                 `json:"image_url"`
	LocationName    string                 `json:"location_name"`
	LocationCity    string                 `json:"location_city"`
	LocationCountry string                 `json:"location_country"`
	LocationAddress string                 `json:"location_address"`
	OrganizerName   string                 `json:"organizer_name"`
	InfoPanels      map[string]interface{} `json:"info_panels,omitempty"`
	InfoPageBlocks  interface{}            `json:"info_page_blocks,omitempty"`
}

type eventJSONLD struct {
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

// FetchEventDetails loads event details for the given event ID or URL.
func (s *Scraper) FetchEventDetails(eventID string, eventURL string) (*EventDetails, error) {
	if eventID == "" && eventURL == "" {
		return nil, fmt.Errorf("event_id or event_url is required")
	}

	if eventURL == "" {
		subdomain := s.DetectEventSubdomain(eventID)
		eventURL = fmt.Sprintf("https://%s/en/event/%s", subdomain, eventID)
	}

	if eventID == "" {
		eventID = ExtractIDFromURL(eventURL)
	}
	if eventID == "" {
		return nil, fmt.Errorf("failed to resolve event_id from event_url")
	}

	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest("GET", eventURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating event request: %w", err)
	}
	req.Header.Set("User-Agent", s.config.Scraper.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching event page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("event page returned status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing event HTML: %w", err)
	}

	details := &EventDetails{
		EventID:  eventID,
		EventURL: eventURL,
	}

	if ld := parseEventJSONLD(doc); ld != nil {
		details.Name = ld.Name
		details.Description = ld.Description
		details.StartDate = ld.StartDate
		details.EndDate = ld.EndDate
		details.ImageURL = ld.Image
		details.LocationName = ld.Location.Name
		details.LocationCity = ld.Location.Address.AddressLocality
		details.LocationCountry = ld.Location.Address.AddressCountry
		details.LocationAddress = ld.Location.Address.Description
		details.OrganizerName = ld.Organizer.Name
	}

	if infoPanels, err := s.fetchEventInfoPanels(eventURL, eventID); err == nil {
		details.InfoPanels = infoPanels
		if details.LocationCity == "" {
			if city, ok := infoPanels["location_city"].(string); ok {
				details.LocationCity = city
			}
		}
		if details.LocationCountry == "" {
			if country, ok := infoPanels["location_country_human"].(string); ok {
				details.LocationCountry = country
			}
		}
		if details.LocationName == "" {
			if name, ok := infoPanels["location_name"].(string); ok {
				details.LocationName = name
			}
		}
		if details.LocationAddress == "" {
			if addr, ok := infoPanels["location_address"].(string); ok {
				details.LocationAddress = addr
			}
		}
		if details.OrganizerName == "" {
			if org, ok := infoPanels["organizer"].(map[string]interface{}); ok {
				if name, ok := org["name"].(string); ok {
					details.OrganizerName = name
				}
			}
		}
	}

	if blocks, err := s.fetchEventInfoBlocks(eventURL, eventID); err == nil {
		if value, ok := blocks["infoPageBlocks"].(interface{}); ok {
			details.InfoPageBlocks = value
		} else {
			details.InfoPageBlocks = blocks
		}
	}

	return details, nil
}

func parseEventJSONLD(doc *goquery.Document) *eventJSONLD {
	var result *eventJSONLD

	doc.Find("script[type='application/ld+json']").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		raw := strings.TrimSpace(s.Text())
		if raw == "" {
			return true
		}

		var single eventJSONLD
		if err := json.Unmarshal([]byte(raw), &single); err == nil && isSportsEvent(single.Type) {
			result = &single
			return false
		}

		var list []eventJSONLD
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
	return strings.EqualFold(strings.TrimSpace(eventType), "SportsEvent")
}

func (s *Scraper) fetchEventInfoPanels(eventURL string, eventID string) (map[string]interface{}, error) {
	endpoint, err := buildEventEndpoint(eventURL, eventID, "getInfoPanelsData")
	if err != nil {
		return nil, err
	}

	return fetchJSON(endpoint, s.config.Scraper.UserAgent)
}

func (s *Scraper) fetchEventInfoBlocks(eventURL string, eventID string) (map[string]interface{}, error) {
	endpoint, err := buildEventEndpoint(eventURL, eventID, "getCmsData")
	if err != nil {
		return nil, err
	}

	return fetchJSON(endpoint, s.config.Scraper.UserAgent)
}

func buildEventEndpoint(eventURL string, eventID string, suffix string) (string, error) {
	parsed, err := url.Parse(eventURL)
	if err != nil {
		return "", fmt.Errorf("invalid event URL: %w", err)
	}

	host := parsed.Scheme + "//" + parsed.Host
	return fmt.Sprintf("%s/en/event/%s/%s", host, eventID, suffix), nil
}

func fetchJSON(endpoint string, userAgent string) (map[string]interface{}, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("endpoint returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("error decoding JSON: %w", err)
	}

	return payload, nil
}

func (s *Scraper) SaveEventDetails(details *EventDetails) error {
	if details == nil {
		return fmt.Errorf("event details is nil")
	}
	if details.EventID == "" {
		return fmt.Errorf("event_id is required")
	}

	infoPanelsJSON, err := marshalJSONString(details.InfoPanels)
	if err != nil {
		return fmt.Errorf("error encoding info panels: %w", err)
	}
	infoBlocksJSON, err := marshalJSONString(details.InfoPageBlocks)
	if err != nil {
		return fmt.Errorf("error encoding info page blocks: %w", err)
	}

	record := models.EventDetail{
		EventID:            details.EventID,
		EventURL:           details.EventURL,
		Name:               details.Name,
		Description:        details.Description,
		StartDate:          details.StartDate,
		EndDate:            details.EndDate,
		ImageURL:           details.ImageURL,
		LocationName:       details.LocationName,
		LocationCity:       details.LocationCity,
		LocationCountry:    details.LocationCountry,
		LocationAddress:    details.LocationAddress,
		OrganizerName:      details.OrganizerName,
		InfoPanelsJSON:     infoPanelsJSON,
		InfoPageBlocksJSON: infoBlocksJSON,
		ScrapedAt:          time.Now(),
	}

	db := config.GetDB()
	var existing models.EventDetail

	query := db.Where("event_id = ?", details.EventID)
	if details.EventURL != "" {
		query = query.Or("event_url = ?", details.EventURL)
	}

	result := query.First(&existing)
	if result.Error == nil {
		record.ID = existing.ID
		record.CreatedAt = existing.CreatedAt
		if err := db.Save(&record).Error; err != nil {
			return fmt.Errorf("failed to update event details: %w", err)
		}
		return nil
	}

	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check event details: %w", result.Error)
	}

	if err := db.Create(&record).Error; err != nil {
		return fmt.Errorf("failed to create event details: %w", err)
	}

	return nil
}

func marshalJSONString(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

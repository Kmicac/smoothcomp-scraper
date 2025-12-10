package scraper

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/kmicac/smoothcomp-scraper/internal/config"
	"github.com/kmicac/smoothcomp-scraper/internal/models"
	"github.com/kmicac/smoothcomp-scraper/pkg/logger"
	"go.uber.org/zap"
)

// ScrapeAcademiesByCountry scrapes academies from a specific country
func (s *Scraper) ScrapeAcademiesByCountry(countryCode string) ([]models.Academy, error) {

	countryName := config.GetCountryName(countryCode)
	logger.Info("Scraping academies",
		zap.String("country", countryCode),
		zap.String("country_name", countryName))

	var academies []models.Academy

	// Create a new collector for this country
	c := s.collector.Clone()

	// Set up the collector to scrape academy listings
	c.OnHTML("a[href*='/club/']", func(e *colly.HTMLElement) {
		academyURL := e.Request.AbsoluteURL(e.Attr("href"))

		// Skip if not a valid academy URL
		if !strings.Contains(academyURL, "/en/club/") || strings.Contains(academyURL, "/finder") {
			return
		}

		// Extract academy ID from URL
		externalID := ExtractIDFromURL(academyURL)
		if externalID == "" {
			return
		}

		// Get academy name
		name := strings.TrimSpace(e.Text)
		if name == "" {
			name = e.ChildText("img[alt]")
		}

		logger.Debug("Found academy",
			zap.String("name", name),
			zap.String("id", externalID),
			zap.String("url", academyURL))

		// Scrape detailed academy info
		academy, err := s.scrapeAcademyDetails(academyURL, externalID, countryCode)
		if err != nil {
			logger.Error("Failed to scrape academy details",
				zap.String("academy", name),
				zap.Error(err))
			return
		}

		if academy != nil {
			academies = append(academies, *academy)
		}
	})

	c.OnError(func(r *colly.Response, err error) {
		logger.Error("Scraping error",
			zap.String("url", r.Request.URL.String()),
			zap.Error(err))
	})

	// Visit the academies page filtered by country
	// We'll start with the general club page and filter later
	url := fmt.Sprintf("%s/en/club", s.config.Scraper.BaseURL)

	logger.Info("Visiting URL", zap.String("url", url))

	if err := c.Visit(url); err != nil {
		return nil, fmt.Errorf("failed to visit academies page: %w", err)
	}

	c.Wait()

	logger.Info("Finished scraping academies",
		zap.String("country", countryCode),
		zap.Int("count", len(academies)))

	return academies, nil
}

// scrapeAcademyDetails scrapes detailed information from an academy page
func (s *Scraper) scrapeAcademyDetails(url, externalID, countryCode string) (*models.Academy, error) {
	logger.Debug("Scraping academy details", zap.String("url", url))

	var academy models.Academy
	academy.ExternalID = externalID
	academy.CountryCode = countryCode
	academy.ScrapedAt = time.Now()

	c := s.collector.Clone()

	c.OnHTML("body", func(e *colly.HTMLElement) {
		// Extract academy name
		academy.Name = strings.TrimSpace(e.ChildText("h1"))
		if academy.Name == "" {
			academy.Name = strings.TrimSpace(e.ChildText(".club-name"))
		}

		// Extract logo URL
		logoURL := e.ChildAttr("img.club-logo", "src")
		if logoURL != "" {
			academy.LogoURL = e.Request.AbsoluteURL(logoURL)
		}

		// Extract cover/banner URL
		coverURL := e.ChildAttr("img.club-cover", "src")
		if coverURL != "" {
			academy.CoverURL = e.Request.AbsoluteURL(coverURL)
		}

		// Extract bio/description
		academy.Bio = strings.TrimSpace(e.ChildText(".club-bio, .club-description"))

		// Extract statistics
		e.ForEach(".stat-item, .stats-item", func(_ int, stat *colly.HTMLElement) {
			label := strings.ToLower(strings.TrimSpace(stat.ChildText(".stat-label, .label")))
			valueStr := strings.TrimSpace(stat.ChildText(".stat-value, .value"))
			value, _ := strconv.Atoi(strings.ReplaceAll(valueStr, ",", ""))

			switch {
			case strings.Contains(label, "wins"):
				academy.TotalWins = value
			case strings.Contains(label, "losses"):
				academy.TotalLosses = value
			case strings.Contains(label, "athletes") || strings.Contains(label, "members"):
				academy.AthleteCount = value
			case strings.Contains(label, "gold"):
				academy.GoldMedals = value
			case strings.Contains(label, "silver"):
				academy.SilverMedals = value
			case strings.Contains(label, "bronze"):
				academy.BronzeMedals = value
			}
		})

		// Extract social links
		academy.Website = e.ChildAttr("a[href*='http']:not([href*='smoothcomp'])", "href")
		academy.Instagram = e.ChildAttr("a[href*='instagram.com']", "href")
		academy.Facebook = e.ChildAttr("a[href*='facebook.com']", "href")

		// Generate slug from name
		academy.Slug = generateSlug(academy.Name)
	})

	c.OnError(func(r *colly.Response, err error) {
		logger.Error("Error scraping academy details", zap.Error(err))
	})

	if err := c.Visit(url); err != nil {
		return nil, err
	}

	c.Wait()

	// Validate we got at least a name
	if academy.Name == "" {
		return nil, fmt.Errorf("failed to extract academy name from %s", url)
	}

	logger.Debug("Academy details scraped",
		zap.String("name", academy.Name),
		zap.Int("wins", academy.TotalWins),
		zap.Int("athletes", academy.AthleteCount))

	return &academy, nil
}

// SaveAcademy saves or updates an academy in the database
func (s *Scraper) SaveAcademy(academy *models.Academy) error {
	db := config.GetDB()

	// Check if academy already exists
	var existing models.Academy
	result := db.Where("external_id = ?", academy.ExternalID).First(&existing)

	if result.Error == nil {
		// Update existing academy
		academy.ID = existing.ID
		academy.CreatedAt = existing.CreatedAt
		if err := db.Save(academy).Error; err != nil {
			return fmt.Errorf("failed to update academy: %w", err)
		}
		logger.Debug("Academy updated", zap.String("name", academy.Name))
	} else {
		// Create new academy
		if err := db.Create(academy).Error; err != nil {
			return fmt.Errorf("failed to create academy: %w", err)
		}
		logger.Debug("Academy created", zap.String("name", academy.Name))
	}

	return nil
}

// generateSlug creates a URL-friendly slug from a name
func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "/", "-")
	// Remove special characters
	var result strings.Builder
	for _, char := range slug {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' {
			result.WriteRune(char)
		}
	}
	return result.String()
}

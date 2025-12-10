package scraper

import (
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/kmicac/smoothcomp-scraper/internal/config"
	"github.com/kmicac/smoothcomp-scraper/internal/models"
	"github.com/kmicac/smoothcomp-scraper/pkg/logger"
	"go.uber.org/zap"
)

type Scraper struct {
	config    *config.Config
	collector *colly.Collector
}

// NewScraper creates a new scraper instance
func NewScraper(cfg *config.Config) *Scraper {
	c := colly.NewCollector(
		colly.UserAgent(cfg.Scraper.UserAgent),
		colly.AllowedDomains("smoothcomp.com", "www.smoothcomp.com"),
	)

	// Set request delay
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*smoothcomp.com*",
		Delay:       time.Duration(cfg.Scraper.RequestDelayMs) * time.Millisecond,
		RandomDelay: 1 * time.Second,
	})

	return &Scraper{
		config:    cfg,
		collector: c,
	}
}

// ScrapeAll scrapes both academies and athletes
func (s *Scraper) ScrapeAll() error {
	logger.Info("Starting full scraping job")

	job := s.createJob("all")

	// Scrape academies first
	if err := s.ScrapeAcademies(); err != nil {
		s.failJob(job, err)
		return err
	}

	// Then scrape athletes
	if err := s.ScrapeAthletes(); err != nil {
		s.failJob(job, err)
		return err
	}

	s.completeJob(job)
	logger.Info("Full scraping completed successfully")
	return nil
}

// ScrapeAcademies scrapes academy data from SmoothComp
func (s *Scraper) ScrapeAcademies() error {
	logger.Info("Starting academy scraping")

	job := s.createJob("academies")
	itemsScraped := 0

	// Scrape academies for each target country
	for _, countryCode := range s.config.Scraper.TargetCountries {
		logger.Info("Scraping country", zap.String("country", countryCode))

		academies, err := s.ScrapeAcademiesByCountry(countryCode)
		if err != nil {
			logger.Error("Failed to scrape country",
				zap.String("country", countryCode),
				zap.Error(err))
			continue
		}

		// Save each academy to database
		for i := range academies {
			if err := s.SaveAcademy(&academies[i]); err != nil {
				logger.Error("Failed to save academy",
					zap.String("academy", academies[i].Name),
					zap.Error(err))
				continue
			}
			itemsScraped++
		}

		logger.Info("Country scraping completed",
			zap.String("country", countryCode),
			zap.Int("academies", len(academies)))
	}

	job.ItemsScraped = itemsScraped
	s.completeJob(job)

	logger.Info("Academy scraping completed", zap.Int("total", itemsScraped))
	return nil
}

// ScrapeAthletes scrapes athlete data from SmoothComp
func (s *Scraper) ScrapeAthletes() error {
	logger.Info("Starting athlete scraping")

	job := s.createJob("athletes")

	// TODO: Implement actual scraping logic
	// For now, this is a placeholder
	logger.Info("Athlete scraping placeholder - will implement actual logic next")

	s.completeJob(job)
	return nil
}

// createJob creates a new scrape job record
func (s *Scraper) createJob(jobType string) *models.ScrapeJob {
	db := config.GetDB()

	job := &models.ScrapeJob{
		JobType:   jobType,
		Status:    "running",
		StartedAt: time.Now(),
	}

	db.Create(job)

	logger.Info("Scrape job created",
		zap.Int("job_id", job.ID),
		zap.String("type", jobType))

	return job
}

// completeJob marks a job as completed
func (s *Scraper) completeJob(job *models.ScrapeJob) {
	db := config.GetDB()

	now := time.Now()
	job.Status = "completed"
	job.CompletedAt = &now

	db.Save(job)

	logger.Info("Scrape job completed",
		zap.Int("job_id", job.ID),
		zap.Int("items_scraped", job.ItemsScraped))
}

// failJob marks a job as failed
func (s *Scraper) failJob(job *models.ScrapeJob, err error) {
	db := config.GetDB()

	now := time.Now()
	job.Status = "failed"
	job.CompletedAt = &now
	job.ErrorMessage = err.Error()

	db.Save(job)

	logger.Error("Scrape job failed",
		zap.Int("job_id", job.ID),
		zap.Error(err))
}

// Helper function to extract ID from SmoothComp URL
func ExtractIDFromURL(url string) string {
	// Split by "/" and get the last part
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// Helper function to map country codes
func GetCountryCode(countryName string) string {
	countryMap := map[string]string{
		"Argentina": "AR",
		"Brazil":    "BR",
		"Brasil":    "BR",
		"Chile":     "CL",
		"Mexico":    "MX",
		"México":    "MX",
		"Ecuador":   "EC",
		"Venezuela": "VE",
		"Peru":      "PE",
		"Perú":      "PE",
		"Colombia":  "CO",
	}

	if code, ok := countryMap[countryName]; ok {
		return code
	}
	return ""
}

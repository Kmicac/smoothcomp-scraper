package scheduler

import (
	"sync"
	"time"

	"github.com/kmicac/smoothcomp-scraper/internal/config"
	"github.com/kmicac/smoothcomp-scraper/internal/scraper"
	"github.com/kmicac/smoothcomp-scraper/pkg/logger"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type Scheduler struct {
	cron      *cron.Cron
	config    *config.Config
	scraper   *scraper.Scraper
	isRunning bool
	mu        sync.RWMutex
	entryID   cron.EntryID
}

// NewScheduler creates a new scheduler instance
func NewScheduler(cfg *config.Config) *Scheduler {
	return &Scheduler{
		cron:      cron.New(),
		config:    cfg,
		scraper:   scraper.NewScraper(cfg),
		isRunning: false,
	}
}

// Start starts the scheduler
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get schedule config from database
	db := config.GetDB()
	var scheduleConfig struct {
		CronExpr string
		Enabled  bool
	}

	db.Table("schedule_configs").First(&scheduleConfig)

	if !scheduleConfig.Enabled {
		logger.Info("Scheduler is disabled")
		return nil
	}

	// Add cron job
	entryID, err := s.cron.AddFunc(scheduleConfig.CronExpr, func() {
		logger.Info("Starting scheduled scraping job")
		s.runScrapingJob()
	})

	if err != nil {
		return err
	}

	s.entryID = entryID
	s.cron.Start()

	logger.Info("Scheduler started successfully",
		zap.String("schedule", scheduleConfig.CronExpr))

	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cron != nil {
		s.cron.Stop()
		logger.Info("Scheduler stopped")
	}
}

// UpdateSchedule updates the cron schedule
func (s *Scheduler) UpdateSchedule(cronExpr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove old schedule
	if s.entryID != 0 {
		s.cron.Remove(s.entryID)
	}

	// Add new schedule
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		logger.Info("Starting scheduled scraping job")
		s.runScrapingJob()
	})

	if err != nil {
		return err
	}

	s.entryID = entryID
	logger.Info("Schedule updated", zap.String("new_schedule", cronExpr))

	return nil
}

// IsRunning returns whether a scraping job is currently running
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}

// GetNextRun returns the next scheduled run time
func (s *Scheduler) GetNextRun() *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.entryID == 0 {
		return nil
	}

	entry := s.cron.Entry(s.entryID)
	nextRun := entry.Next
	return &nextRun
}

// runScrapingJob executes the scraping job
func (s *Scheduler) runScrapingJob() {
	s.mu.Lock()
	if s.isRunning {
		logger.Warn("Scraping job already running, skipping this execution")
		s.mu.Unlock()
		return
	}
	s.isRunning = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isRunning = false
		s.mu.Unlock()
	}()

	logger.Info("Executing scheduled scraping job")

	// Run scraping
	if err := s.scraper.ScrapeAll(); err != nil {
		logger.Error("Scheduled scraping job failed", zap.Error(err))
		return
	}

	logger.Info("Scheduled scraping job completed successfully")
}

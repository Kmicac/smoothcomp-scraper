package config

import (
	"fmt"

	"github.com/kmicac/smoothcomp-scraper/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDatabase initializes the SQLite database connection
func InitDatabase(dbPath string) error {
	var err error

	// Configure GORM
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	// Open SQLite connection
	DB, err = gorm.Open(sqlite.Open(dbPath), gormConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate models
	err = DB.AutoMigrate(
		&models.Academy{},
		&models.Athlete{},
		&models.ScrapeJob{},
		&models.ScheduleConfig{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	// Initialize default schedule config if not exists
	var scheduleConfig models.ScheduleConfig
	result := DB.First(&scheduleConfig)
	if result.Error == gorm.ErrRecordNotFound {
		defaultSchedule := models.ScheduleConfig{
			CronExpr: "0 2 1 * *", // 1st day of month at 2 AM (Monthly)
			Enabled:  true,
		}
		if err := DB.Create(&defaultSchedule).Error; err != nil {
			return fmt.Errorf("failed to create default schedule: %w", err)
		}
	}

	return nil
}

// GetDB returns the database instance
func GetDB() *gorm.DB {
	return DB
}

// CloseDatabase closes the database connection
func CloseDatabase() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

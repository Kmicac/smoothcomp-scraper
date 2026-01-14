package config

import (
	"fmt"

	"github.com/kmicac/smoothcomp-scraper/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDatabase(dbPath string) error {
	var err error

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	DB, err = gorm.Open(sqlite.Open(dbPath), gormConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	err = DB.AutoMigrate(
		&models.Academy{},
		&models.Athlete{},
		&models.Event{},
		&models.EventDetail{},
		&models.EventRegistration{},
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

func GetDB() *gorm.DB {
	return DB
}

func CloseDatabase() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

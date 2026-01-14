package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig
	Scraper   ScraperConfig
	Scheduler SchedulerConfig
	Database  DatabaseConfig
	Logging   LoggingConfig
}

type ServerConfig struct {
	Port        string
	Environment string
}

type ScraperConfig struct {
	BaseURL           string
	UserAgent         string
	RequestDelayMs    int
	MaxRetries        int
	RateLimitRequests int
	RateLimitDuration time.Duration
	TargetCountries   []string
}

type SchedulerConfig struct {
	CronExpression string
	Enabled        bool
}

type DatabaseConfig struct {
	CachePath string
}

type LoggingConfig struct {
	Level string
}

// LoadConfig loads configuration from environment variables and .env file
func LoadConfig() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	// Attempt to read .env file (optional)
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Warning: .env file not found, using environment variables: %v\n", err)
	}

	viper.SetDefault("PORT", "8080")
	viper.SetDefault("ENVIRONMENT", "development")
	viper.SetDefault("SMOOTHCOMP_BASE_URL", "https://smoothcomp.com")
	viper.SetDefault("USER_AGENT", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	viper.SetDefault("REQUEST_DELAY_MS", 2000)
	viper.SetDefault("MAX_RETRIES", 3)
	viper.SetDefault("RATE_LIMIT_REQUESTS", 10)
	viper.SetDefault("RATE_LIMIT_DURATION", 60)
	viper.SetDefault("SCHEDULE_CRON", "0 2 * * 0") // Every Sunday at 2 AM
	viper.SetDefault("TARGET_COUNTRIES", "AR,BR,CL,MX,EC,VE,PE,CO")
	viper.SetDefault("CACHE_DB_PATH", "./storage/cache.db")
	viper.SetDefault("LOG_LEVEL", "info")

	config := &Config{
		Server: ServerConfig{
			Port:        viper.GetString("PORT"),
			Environment: viper.GetString("ENVIRONMENT"),
		},
		Scraper: ScraperConfig{
			BaseURL:           viper.GetString("SMOOTHCOMP_BASE_URL"),
			UserAgent:         viper.GetString("USER_AGENT"),
			RequestDelayMs:    viper.GetInt("REQUEST_DELAY_MS"),
			MaxRetries:        viper.GetInt("MAX_RETRIES"),
			RateLimitRequests: viper.GetInt("RATE_LIMIT_REQUESTS"),
			RateLimitDuration: time.Duration(viper.GetInt("RATE_LIMIT_DURATION")) * time.Second,
			TargetCountries:   parseCountries(viper.GetString("TARGET_COUNTRIES")),
		},
		Scheduler: SchedulerConfig{
			CronExpression: viper.GetString("SCHEDULE_CRON"),
			Enabled:        true,
		},
		Database: DatabaseConfig{
			CachePath: viper.GetString("CACHE_DB_PATH"),
		},
		Logging: LoggingConfig{
			Level: viper.GetString("LOG_LEVEL"),
		},
	}

	return config, nil
}

// parseCountries splits comma-separated country codes
func parseCountries(countriesStr string) []string {
	if countriesStr == "" {
		return []string{}
	}

	countries := strings.Split(countriesStr, ",")
	result := make([]string, 0, len(countries))

	for _, country := range countries {
		trimmed := strings.TrimSpace(country)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// GetCountryName returns the full country name from country code
func GetCountryName(code string) string {
	countryMap := map[string]string{
		"AR": "Argentina",
		"BR": "Brazil",
		"CL": "Chile",
		"MX": "Mexico",
		"EC": "Ecuador",
		"VE": "Venezuela",
		"PE": "Peru",
		"CO": "Colombia",
	}

	if name, ok := countryMap[code]; ok {
		return name
	}
	return code
}

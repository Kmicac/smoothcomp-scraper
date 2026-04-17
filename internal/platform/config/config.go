package config

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
)

type Config struct {
	Service    ServiceConfig
	HTTP       HTTPConfig
	Security   SecurityConfig
	Storage    StorageConfig
	Smoothcomp SmoothcompConfig
	Worker     WorkerConfig
	Scheduler  SchedulerConfig
	Logging    LoggingConfig
}

type ServiceConfig struct {
	Name        string
	Environment string
}

type HTTPConfig struct {
	BindAddress     string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

func (c HTTPConfig) Address() string {
	return net.JoinHostPort(c.BindAddress, fmt.Sprintf("%d", c.Port))
}

type SecurityConfig struct {
	InternalToken      string
	AllowInsecureAuth  bool
	CorrelationHeader  string
	InternalAuthHeader string
}

type StorageConfig struct {
	Driver string
	DSN    string
}

type SmoothcompConfig struct {
	BaseURL         string
	UserAgent       string
	Timeout         time.Duration
	RequestDelay    time.Duration
	TargetCountries []string
	EventTypes      []string
}

type WorkerConfig struct {
	PollInterval time.Duration
}

type SchedulerConfig struct {
	Enabled        bool
	CronExpression string
	Name           string
}

type LoggingConfig struct {
	Level string
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	v.AutomaticEnv()

	_ = v.ReadInConfig()

	v.SetDefault("SERVICE_NAME", "smoothcomp-ingestion-adapter")
	v.SetDefault("ENVIRONMENT", "development")
	v.SetDefault("HTTP_BIND_ADDRESS", "127.0.0.1")
	v.SetDefault("HTTP_PORT", 8080)
	v.SetDefault("HTTP_READ_TIMEOUT_SEC", 15)
	v.SetDefault("HTTP_WRITE_TIMEOUT_SEC", 15)
	v.SetDefault("HTTP_IDLE_TIMEOUT_SEC", 60)
	v.SetDefault("HTTP_SHUTDOWN_TIMEOUT_SEC", 10)
	v.SetDefault("INTERNAL_AUTH_HEADER", "Authorization")
	v.SetDefault("CORRELATION_ID_HEADER", "X-Correlation-ID")
	v.SetDefault("ALLOW_INSECURE_INTERNAL_AUTH", false)
	v.SetDefault("DATABASE_DRIVER", "sqlite")
	v.SetDefault("DATABASE_DSN", "./storage/adapter.db")
	v.SetDefault("SMOOTHCOMP_BASE_URL", "https://smoothcomp.com")
	v.SetDefault("SMOOTHCOMP_USER_AGENT", "smoothcomp-ingestion-adapter/1.0 (+internal)")
	v.SetDefault("SMOOTHCOMP_TIMEOUT_SEC", 20)
	v.SetDefault("SMOOTHCOMP_REQUEST_DELAY_MS", 500)
	v.SetDefault("SMOOTHCOMP_TARGET_COUNTRIES", "AR")
	v.SetDefault("SMOOTHCOMP_EVENT_TYPES", "upcoming,past")
	v.SetDefault("WORKER_POLL_INTERVAL_SEC", 5)
	v.SetDefault("SCHEDULER_ENABLED", false)
	v.SetDefault("SCHEDULER_CRON", "0 2 * * *")
	v.SetDefault("SCHEDULER_NAME", "smoothcomp-default-catalog")
	v.SetDefault("LOG_LEVEL", "info")

	cfg := &Config{
		Service: ServiceConfig{
			Name:        v.GetString("SERVICE_NAME"),
			Environment: strings.ToLower(strings.TrimSpace(v.GetString("ENVIRONMENT"))),
		},
		HTTP: HTTPConfig{
			BindAddress:     strings.TrimSpace(v.GetString("HTTP_BIND_ADDRESS")),
			Port:            v.GetInt("HTTP_PORT"),
			ReadTimeout:     time.Duration(v.GetInt("HTTP_READ_TIMEOUT_SEC")) * time.Second,
			WriteTimeout:    time.Duration(v.GetInt("HTTP_WRITE_TIMEOUT_SEC")) * time.Second,
			IdleTimeout:     time.Duration(v.GetInt("HTTP_IDLE_TIMEOUT_SEC")) * time.Second,
			ShutdownTimeout: time.Duration(v.GetInt("HTTP_SHUTDOWN_TIMEOUT_SEC")) * time.Second,
		},
		Security: SecurityConfig{
			InternalToken:      strings.TrimSpace(v.GetString("INTERNAL_AUTH_TOKEN")),
			AllowInsecureAuth:  v.GetBool("ALLOW_INSECURE_INTERNAL_AUTH"),
			CorrelationHeader:  strings.TrimSpace(v.GetString("CORRELATION_ID_HEADER")),
			InternalAuthHeader: strings.TrimSpace(v.GetString("INTERNAL_AUTH_HEADER")),
		},
		Storage: StorageConfig{
			Driver: strings.ToLower(strings.TrimSpace(v.GetString("DATABASE_DRIVER"))),
			DSN:    strings.TrimSpace(v.GetString("DATABASE_DSN")),
		},
		Smoothcomp: SmoothcompConfig{
			BaseURL:         strings.TrimRight(strings.TrimSpace(v.GetString("SMOOTHCOMP_BASE_URL")), "/"),
			UserAgent:       strings.TrimSpace(v.GetString("SMOOTHCOMP_USER_AGENT")),
			Timeout:         time.Duration(v.GetInt("SMOOTHCOMP_TIMEOUT_SEC")) * time.Second,
			RequestDelay:    time.Duration(v.GetInt("SMOOTHCOMP_REQUEST_DELAY_MS")) * time.Millisecond,
			TargetCountries: normalizeCSV(v.GetString("SMOOTHCOMP_TARGET_COUNTRIES"), true),
			EventTypes:      normalizeCSV(v.GetString("SMOOTHCOMP_EVENT_TYPES"), false),
		},
		Worker: WorkerConfig{
			PollInterval: time.Duration(v.GetInt("WORKER_POLL_INTERVAL_SEC")) * time.Second,
		},
		Scheduler: SchedulerConfig{
			Enabled:        v.GetBool("SCHEDULER_ENABLED"),
			CronExpression: strings.TrimSpace(v.GetString("SCHEDULER_CRON")),
			Name:           strings.TrimSpace(v.GetString("SCHEDULER_NAME")),
		},
		Logging: LoggingConfig{
			Level: strings.ToLower(strings.TrimSpace(v.GetString("LOG_LEVEL"))),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Service.Name == "" {
		return fmt.Errorf("service name is required")
	}
	if c.HTTP.BindAddress == "" {
		return fmt.Errorf("http bind address is required")
	}
	if c.HTTP.Port <= 0 || c.HTTP.Port > 65535 {
		return fmt.Errorf("http port must be between 1 and 65535")
	}
	if !c.Security.AllowInsecureAuth && c.Security.InternalToken == "" {
		return fmt.Errorf("INTERNAL_AUTH_TOKEN is required unless ALLOW_INSECURE_INTERNAL_AUTH=true")
	}
	switch c.Storage.Driver {
	case "sqlite", "postgres":
	default:
		return fmt.Errorf("unsupported DATABASE_DRIVER %q", c.Storage.Driver)
	}
	if c.Storage.DSN == "" {
		return fmt.Errorf("database dsn is required")
	}
	if c.Smoothcomp.BaseURL == "" {
		return fmt.Errorf("smoothcomp base url is required")
	}
	if c.Smoothcomp.UserAgent == "" {
		return fmt.Errorf("smoothcomp user agent is required")
	}
	if len(c.Smoothcomp.TargetCountries) == 0 {
		return fmt.Errorf("at least one Smoothcomp target country is required")
	}
	for _, eventType := range c.Smoothcomp.EventTypes {
		if eventType != "past" && eventType != "upcoming" {
			return fmt.Errorf("unsupported Smoothcomp event type %q", eventType)
		}
	}
	if c.Scheduler.Enabled {
		if c.Scheduler.CronExpression == "" {
			return fmt.Errorf("scheduler cron is required when scheduler is enabled")
		}
		if _, err := cron.ParseStandard(c.Scheduler.CronExpression); err != nil {
			return fmt.Errorf("invalid scheduler cron: %w", err)
		}
	}
	return nil
}

func normalizeCSV(value string, upper bool) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if upper {
			trimmed = strings.ToUpper(trimmed)
		} else {
			trimmed = strings.ToLower(trimmed)
		}
		items = append(items, trimmed)
	}
	return items
}

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kmicac/smoothcomp-scraper/internal/api"
	"github.com/kmicac/smoothcomp-scraper/internal/config"
	"github.com/kmicac/smoothcomp-scraper/internal/scheduler"
	"github.com/kmicac/smoothcomp-scraper/pkg/logger"
	"go.uber.org/zap"
)

const Version = "1.0.0"

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	if err := logger.InitLogger(cfg.Logging.Level); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting SmoothComp Scraper Service",
		zap.String("version", Version),
		zap.String("environment", cfg.Server.Environment),
	)

	// Initialize database
	if err := config.InitDatabase(cfg.Database.CachePath); err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer config.CloseDatabase()

	logger.Info("Database initialized successfully")

	// Initialize scheduler
	cronScheduler := scheduler.NewScheduler(cfg)
	if cfg.Scheduler.Enabled {
		if err := cronScheduler.Start(); err != nil {
			logger.Fatal("Failed to start scheduler", zap.Error(err))
		}
		logger.Info("Scheduler started", zap.String("cron", cfg.Scheduler.CronExpression))
	}

	// Initialize HTTP router
	router := api.NewRouter(cfg, cronScheduler)

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("HTTP server listening", zap.String("port", cfg.Server.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Stop scheduler
	cronScheduler.Stop()

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server stopped gracefully")
}

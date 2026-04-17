package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	platformbootstrap "github.com/kmicac/smoothcomp-scraper/internal/platform/bootstrap"
	platformconfig "github.com/kmicac/smoothcomp-scraper/internal/platform/config"
)

// cmd/server remains as a local-dev compatibility binary that runs API and worker in one process.
func main() {
	cfg, err := platformconfig.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	runtime, err := platformbootstrap.NewRuntime(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to bootstrap runtime: %v\n", err)
		os.Exit(1)
	}
	defer runtime.Close(context.Background())

	server := &http.Server{
		Addr:         cfg.HTTP.Address(),
		Handler:      runtime.Router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	runtime.Logger.Warn("cmd/server is a local convenience binary; use cmd/api and cmd/worker as the supported runtime topology")

	if err := runtime.Scheduler.Start(); err != nil {
		runtime.Logger.Fatal("scheduler failed to start", runtime.ErrField(err))
	}
	defer runtime.Scheduler.Stop()

	go func() {
		if err := runtime.Worker.Run(ctx); err != nil {
			runtime.Logger.Fatal("worker failed", runtime.ErrField(err))
		}
	}()

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			runtime.Logger.Fatal("http server failed", runtime.ErrField(err))
		}
	}()

	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		runtime.Logger.Error("http shutdown failed", runtime.ErrField(err))
	}
}

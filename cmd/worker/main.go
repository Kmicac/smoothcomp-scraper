package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	platformbootstrap "github.com/kmicac/smoothcomp-scraper/internal/platform/bootstrap"
	platformconfig "github.com/kmicac/smoothcomp-scraper/internal/platform/config"
)

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

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := runtime.Scheduler.Start(); err != nil {
		runtime.Logger.Fatal("scheduler failed to start", runtime.ErrField(err))
	}
	defer runtime.Scheduler.Stop()

	runtime.Logger.Info("starting ingestion worker", runtime.LogField("worker_id", runtime.Worker.WorkerID()))

	if err := runtime.Worker.Run(ctx); err != nil {
		runtime.Logger.Fatal("worker failed", runtime.ErrField(err))
	}
}

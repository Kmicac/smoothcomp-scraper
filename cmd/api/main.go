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

	runtime.Logger.Info("starting internal API", runtime.LogField("address", server.Addr))

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			runtime.Logger.Fatal("http server failed", runtime.ErrField(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		runtime.Logger.Error("http shutdown failed", runtime.ErrField(err))
	}
}

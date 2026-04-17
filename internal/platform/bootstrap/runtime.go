package bootstrap

import (
	"context"
	"fmt"
	"net/http"

	"github.com/kmicac/smoothcomp-scraper/internal/adapters/smoothcomp"
	"github.com/kmicac/smoothcomp-scraper/internal/adapters/storage/gormstore"
	"github.com/kmicac/smoothcomp-scraper/internal/adapters/transport/httpapi"
	"github.com/kmicac/smoothcomp-scraper/internal/application/ingestion"
	"github.com/kmicac/smoothcomp-scraper/internal/application/operations"
	appscheduler "github.com/kmicac/smoothcomp-scraper/internal/application/scheduler"
	platformconfig "github.com/kmicac/smoothcomp-scraper/internal/platform/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Runtime struct {
	Logger    *zap.Logger
	Router    http.Handler
	Worker    *ingestion.Worker
	Scheduler *appscheduler.Service
	store     *gormstore.Store
}

func NewRuntime(cfg *platformconfig.Config) (*Runtime, error) {
	logger, err := newLogger(cfg)
	if err != nil {
		return nil, err
	}

	store, err := gormstore.Open(cfg.Storage)
	if err != nil {
		_ = logger.Sync()
		return nil, err
	}

	repos := gormstore.NewRepositories(store)
	client := smoothcomp.NewClient(cfg.Smoothcomp, logger)
	registry := ingestion.NewRegistry(
		smoothcomp.NewEventCatalogPipeline(client),
		smoothcomp.NewEventParticipantsPipeline(client),
	)

	commands := ingestion.NewCommandService(repos, registry, logger)
	worker := ingestion.NewWorker(repos, repos, repos, registry, logger, cfg.Worker.PollInterval)
	ops := operations.NewService(repos, repos, repos)
	scheduler := appscheduler.NewService(cfg.Scheduler, cfg.Smoothcomp, commands, repos, logger)
	router := httpapi.NewRouter(cfg, logger, ops, commands)

	return &Runtime{
		Logger:    logger,
		Router:    router,
		Worker:    worker,
		Scheduler: scheduler,
		store:     store,
	}, nil
}

func (r *Runtime) Close(_ context.Context) error {
	var closeErr error
	if r.store != nil {
		closeErr = r.store.Close()
	}
	if r.Logger != nil {
		_ = r.Logger.Sync()
	}
	return closeErr
}

func (r *Runtime) LogField(key string, value any) zap.Field {
	return zap.Any(key, value)
}

func (r *Runtime) ErrField(err error) zap.Field {
	return zap.Error(err)
}

func newLogger(cfg *platformconfig.Config) (*zap.Logger, error) {
	level := zapcore.InfoLevel
	if err := level.Set(cfg.Logging.Level); err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	zapCfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Development:      cfg.Service.Environment == "development",
		Encoding:         "json",
		EncoderConfig:    encoderCfg,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	return zapCfg.Build()
}

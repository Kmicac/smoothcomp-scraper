package scheduler

import (
	"context"
	"fmt"

	"github.com/kmicac/smoothcomp-scraper/internal/application/ingestion"
	"github.com/kmicac/smoothcomp-scraper/internal/core/job"
	"github.com/kmicac/smoothcomp-scraper/internal/core/port"
	platformconfig "github.com/kmicac/smoothcomp-scraper/internal/platform/config"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type Service struct {
	cfg       platformconfig.SchedulerConfig
	smoothCfg platformconfig.SmoothcompConfig
	commands  *ingestion.CommandService
	schedules port.ScheduleRepository
	logger    *zap.Logger
	cron      *cron.Cron
}

func NewService(
	cfg platformconfig.SchedulerConfig,
	smoothCfg platformconfig.SmoothcompConfig,
	commands *ingestion.CommandService,
	schedules port.ScheduleRepository,
	logger *zap.Logger,
) *Service {
	return &Service{
		cfg:       cfg,
		smoothCfg: smoothCfg,
		commands:  commands,
		schedules: schedules,
		logger:    logger,
		cron:      cron.New(),
	}
}

func (s *Service) Start() error {
	if err := s.schedules.Upsert(context.Background(), &job.Schedule{
		Name:           s.cfg.Name,
		CronExpression: s.cfg.CronExpression,
		Enabled:        s.cfg.Enabled,
	}); err != nil {
		return err
	}
	if !s.cfg.Enabled {
		s.logger.Info("scheduler disabled")
		return nil
	}

	if _, err := s.cron.AddFunc(s.cfg.CronExpression, func() {
		s.enqueueDefaultCatalogJobs(context.Background())
	}); err != nil {
		return err
	}

	s.cron.Start()
	s.logger.Info("scheduler started", zap.String("cron", s.cfg.CronExpression))
	return nil
}

func (s *Service) Stop() {
	if s.cron != nil {
		s.cron.Stop()
	}
}

func (s *Service) enqueueDefaultCatalogJobs(ctx context.Context) {
	for _, country := range s.smoothCfg.TargetCountries {
		for _, eventType := range s.smoothCfg.EventTypes {
			request := job.Request{
				Pipeline:      job.PipelineSmoothcompEventCatalog,
				Trigger:       job.TriggerScheduled,
				CorrelationID: fmt.Sprintf("sched-%s-%s", country, eventType),
				Country:       country,
				EventType:     eventType,
				Metadata: map[string]string{
					"schedule_name": s.cfg.Name,
				},
			}
			if _, err := s.commands.Enqueue(ctx, request); err != nil {
				s.logger.Error("scheduled enqueue failed", zap.String("country", country), zap.String("event_type", eventType), zap.Error(err))
			}
		}
	}
}

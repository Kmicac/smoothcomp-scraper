package operations

import (
	"context"

	"github.com/kmicac/smoothcomp-scraper/internal/core/contract"
	"github.com/kmicac/smoothcomp-scraper/internal/core/job"
	"github.com/kmicac/smoothcomp-scraper/internal/core/port"
	corepublication "github.com/kmicac/smoothcomp-scraper/internal/core/publication"
)

type Service struct {
	jobs    port.JobRepository
	results port.ResultRepository
	health  port.HealthRepository
}

func NewService(jobs port.JobRepository, results port.ResultRepository, health port.HealthRepository) *Service {
	return &Service{jobs: jobs, results: results, health: health}
}

func (s *Service) Liveness() map[string]string {
	return map[string]string{"status": "ok"}
}

func (s *Service) Readiness(ctx context.Context) error {
	return s.health.Ping(ctx)
}

func (s *Service) ListJobs(ctx context.Context, limit int) ([]job.Job, error) {
	return s.jobs.List(ctx, port.JobListFilter{Limit: limit})
}

func (s *Service) GetJob(ctx context.Context, id string) (*job.Job, error) {
	return s.jobs.Get(ctx, id)
}

func (s *Service) LatestPublication(ctx context.Context, pipeline job.Pipeline) (*job.PublishedResult, error) {
	return s.results.GetLatestPublished(ctx, pipeline)
}

func (s *Service) LatestPublicationByScope(ctx context.Context, provider string, pipeline job.Pipeline, scope contract.Scope) (*job.PublishedResult, error) {
	return s.results.GetLatestPublishedByScope(ctx, pipeline, corepublication.ScopeKey(provider, pipeline, scope))
}

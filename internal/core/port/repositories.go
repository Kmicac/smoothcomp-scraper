package port

import (
	"context"
	"errors"

	"github.com/kmicac/smoothcomp-scraper/internal/core/contract"
	"github.com/kmicac/smoothcomp-scraper/internal/core/job"
)

var ErrPublicationNotFound = errors.New("publication not found")

type JobListFilter struct {
	Limit int
}

type JobRepository interface {
	Create(context.Context, *job.Job) error
	ClaimNextAvailable(context.Context, job.ClaimOptions) (*job.Job, error)
	Heartbeat(context.Context, job.LeaseHeartbeat) error
	Complete(context.Context, job.Completion) (*job.Job, error)
	Fail(context.Context, job.FailureTransition) (*job.Job, error)
	Get(context.Context, string) (*job.Job, error)
	List(context.Context, JobListFilter) ([]job.Job, error)
}

type SnapshotRepository interface {
	Save(context.Context, *job.RawSnapshot) error
	ListByJob(context.Context, string) ([]job.RawSnapshot, error)
}

type ResultRepository interface {
	SaveNormalized(context.Context, *job.NormalizedResult) error
	SavePublished(context.Context, *job.PublishedResult) error
	GetLatestPublished(context.Context, job.Pipeline) (*job.PublishedResult, error)
	GetLatestPublishedByScope(context.Context, job.Pipeline, string) (*job.PublishedResult, error)
}

type ScheduleRepository interface {
	Upsert(context.Context, *job.Schedule) error
}

type HealthRepository interface {
	Ping(context.Context) error
}

type Pipeline interface {
	Name() job.Pipeline
	Provider() string
	ParserVersion() string
	NormalizationVersion() string
	Fetch(context.Context, job.Request) ([]job.RawSnapshot, error)
	Normalize(context.Context, job.Request, []job.RawSnapshot) (contract.Envelope, error)
	Publish(context.Context, job.Request, contract.Envelope) (contract.Envelope, error)
}

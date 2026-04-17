package ingestion

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kmicac/smoothcomp-scraper/internal/core/contract"
	coreerrors "github.com/kmicac/smoothcomp-scraper/internal/core/errors"
	"github.com/kmicac/smoothcomp-scraper/internal/core/job"
	"github.com/kmicac/smoothcomp-scraper/internal/core/port"
	"go.uber.org/zap"
)

type Registry struct {
	pipelines map[job.Pipeline]port.Pipeline
}

func NewRegistry(pipelines ...port.Pipeline) *Registry {
	items := make(map[job.Pipeline]port.Pipeline, len(pipelines))
	for _, pipeline := range pipelines {
		items[pipeline.Name()] = pipeline
	}
	return &Registry{pipelines: items}
}

func (r *Registry) Get(name job.Pipeline) (port.Pipeline, bool) {
	pipeline, ok := r.pipelines[name]
	return pipeline, ok
}

type CommandService struct {
	jobs     port.JobRepository
	registry *Registry
	logger   *zap.Logger
}

func NewCommandService(jobs port.JobRepository, registry *Registry, logger *zap.Logger) *CommandService {
	return &CommandService{jobs: jobs, registry: registry, logger: logger}
}

func (s *CommandService) Enqueue(ctx context.Context, request job.Request) (*job.Job, error) {
	if request.Pipeline == "" {
		return nil, coreerrors.New(coreerrors.CategoryValidation, coreerrors.CodeInvalidRequest, "ingestion.enqueue", false, "pipeline is required", nil)
	}
	pipeline, ok := s.registry.Get(request.Pipeline)
	if !ok {
		return nil, coreerrors.New(coreerrors.CategoryValidation, coreerrors.CodeInvalidRequest, "ingestion.enqueue", false, "unknown pipeline", nil)
	}
	if request.Trigger == "" {
		request.Trigger = job.TriggerManual
	}

	now := time.Now().UTC()
	record := &job.Job{
		ID:                   job.NewID("job"),
		Provider:             pipeline.Provider(),
		Pipeline:             request.Pipeline,
		Trigger:              request.Trigger,
		State:                job.StatePending,
		CorrelationID:        request.CorrelationID,
		ParserVersion:        pipeline.ParserVersion(),
		NormalizationVersion: pipeline.NormalizationVersion(),
		Request:              request,
		CreatedAt:            now,
	}

	if err := s.jobs.Create(ctx, record); err != nil {
		return nil, coreerrors.New(coreerrors.CategoryStorage, coreerrors.CodeStorageFailed, "ingestion.enqueue", true, "failed to persist job", err)
	}

	s.logger.Info("job enqueued",
		zap.String("job_id", record.ID),
		zap.String("pipeline", string(record.Pipeline)),
		zap.String("correlation_id", record.CorrelationID),
	)

	return record, nil
}

type Worker struct {
	jobs      port.JobRepository
	snapshots port.SnapshotRepository
	results   port.ResultRepository
	registry  *Registry
	logger    *zap.Logger
	workerID  string
	interval  time.Duration
}

func NewWorker(
	jobs port.JobRepository,
	snapshots port.SnapshotRepository,
	results port.ResultRepository,
	registry *Registry,
	logger *zap.Logger,
	pollInterval time.Duration,
) *Worker {
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	return &Worker{
		jobs:      jobs,
		snapshots: snapshots,
		results:   results,
		registry:  registry,
		logger:    logger,
		workerID:  job.NewID("worker"),
		interval:  pollInterval,
	}
}

func (w *Worker) WorkerID() string {
	return w.workerID
}

func (w *Worker) Run(ctx context.Context) error {
	if _, err := w.RunOnce(ctx); err != nil {
		w.logger.Error("initial worker cycle failed", zap.Error(err))
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if _, err := w.RunOnce(ctx); err != nil {
				w.logger.Error("worker cycle failed", zap.Error(err))
			}
		}
	}
}

func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	record, err := w.jobs.ClaimNextPending(ctx, w.workerID)
	if err != nil {
		return false, coreerrors.New(coreerrors.CategoryStorage, coreerrors.CodeStorageFailed, "ingestion.worker.claim", true, "failed to claim job", err)
	}
	if record == nil {
		return false, nil
	}

	pipeline, ok := w.registry.Get(record.Pipeline)
	if !ok {
		failErr := coreerrors.New(coreerrors.CategoryValidation, coreerrors.CodeInvalidRequest, "ingestion.worker.pipeline", false, "pipeline not registered", nil)
		w.failJob(ctx, record, failErr)
		return true, failErr
	}

	w.logger.Info("processing job",
		zap.String("job_id", record.ID),
		zap.String("pipeline", string(record.Pipeline)),
		zap.String("worker_id", w.workerID),
	)

	snapshots, err := pipeline.Fetch(ctx, record.Request)
	if err != nil {
		w.failJob(ctx, record, err)
		return true, err
	}

	for i := range snapshots {
		snapshots[i].JobID = record.ID
		snapshots[i].Provider = record.Provider
		snapshots[i].Pipeline = record.Pipeline
		snapshots[i].ParserVersion = record.ParserVersion
		if err := w.snapshots.Save(ctx, &snapshots[i]); err != nil {
			failErr := coreerrors.New(coreerrors.CategoryStorage, coreerrors.CodeStorageFailed, "ingestion.worker.save_snapshot", true, "failed to store raw snapshot", err)
			w.failJob(ctx, record, failErr)
			return true, failErr
		}
	}

	normalizedPayload, err := pipeline.Normalize(ctx, record.Request, snapshots)
	if err != nil {
		w.failJob(ctx, record, err)
		return true, err
	}
	normalizedPayload.JobID = record.ID

	normalized := &job.NormalizedResult{
		ID:                   job.NewID("norm"),
		JobID:                record.ID,
		Provider:             record.Provider,
		Pipeline:             record.Pipeline,
		ParserVersion:        record.ParserVersion,
		NormalizationVersion: record.NormalizationVersion,
		CreatedAt:            time.Now().UTC(),
		Payload:              normalizedPayload,
	}
	if err := w.results.SaveNormalized(ctx, normalized); err != nil {
		failErr := coreerrors.New(coreerrors.CategoryStorage, coreerrors.CodeStorageFailed, "ingestion.worker.save_normalized", true, "failed to store normalized result", err)
		w.failJob(ctx, record, failErr)
		return true, failErr
	}

	publishedPayload, err := pipeline.Publish(ctx, record.Request, normalizedPayload)
	if err != nil {
		w.failJob(ctx, record, err)
		return true, err
	}
	publishedPayload.JobID = record.ID

	published := &job.PublishedResult{
		ID:              job.NewID("pub"),
		JobID:           record.ID,
		Provider:        record.Provider,
		Pipeline:        record.Pipeline,
		ContractVersion: publishedPayload.ContractVersion,
		PublishedAt:     time.Now().UTC(),
		Payload:         publishedPayload,
	}
	if err := w.results.SavePublished(ctx, published); err != nil {
		failErr := coreerrors.New(coreerrors.CategoryStorage, coreerrors.CodeStorageFailed, "ingestion.worker.save_published", true, "failed to store published result", err)
		w.failJob(ctx, record, failErr)
		return true, failErr
	}

	now := time.Now().UTC()
	record.State = job.StateSucceeded
	record.FinishedAt = &now
	record.Stats = StatsFromEnvelope(len(snapshots), publishedPayload)

	if err := w.jobs.Update(ctx, record); err != nil {
		return true, coreerrors.New(coreerrors.CategoryStorage, coreerrors.CodeStorageFailed, "ingestion.worker.complete", true, "failed to complete job", err)
	}

	w.logger.Info("job completed",
		zap.String("job_id", record.ID),
		zap.String("pipeline", string(record.Pipeline)),
		zap.Int("snapshots", record.Stats.SnapshotCount),
		zap.Int("events", record.Stats.EventCount),
		zap.Int("people", record.Stats.PersonCount),
		zap.Int("registrations", record.Stats.RegistrationCount),
	)

	return true, nil
}

func StatsFromEnvelope(snapshotCount int, payload contract.Envelope) job.Stats {
	return job.Stats{
		SnapshotCount:     snapshotCount,
		EventCount:        len(payload.Events),
		OrganizationCount: len(payload.Organizations),
		PersonCount:       len(payload.People),
		RegistrationCount: len(payload.Registrations),
	}
}

func (w *Worker) failJob(ctx context.Context, record *job.Job, err error) {
	now := time.Now().UTC()
	record.State = job.StateFailed
	record.FinishedAt = &now
	record.Error = failureFromError(err)
	if updateErr := w.jobs.Update(ctx, record); updateErr != nil {
		w.logger.Error("failed to persist failed job state", zap.Error(updateErr), zap.String("job_id", record.ID))
	}
	w.logger.Error("job failed", zap.String("job_id", record.ID), zap.Error(err))
}

func failureFromError(err error) *job.Failure {
	if err == nil {
		return nil
	}
	var typed *coreerrors.Error
	if errors.As(err, &typed) {
		return &job.Failure{
			Category:  string(typed.Category),
			Code:      string(typed.Code),
			Message:   typed.Message,
			Retryable: typed.Retryable,
		}
	}
	return &job.Failure{
		Category:  string(coreerrors.CategoryInternal),
		Code:      string(coreerrors.CodeInternal),
		Message:   err.Error(),
		Retryable: false,
	}
}

func UnsupportedPipelineError(name job.Pipeline) error {
	return coreerrors.New(coreerrors.CategoryValidation, coreerrors.CodeInvalidRequest, "ingestion.pipeline", false, fmt.Sprintf("pipeline %s is not supported", name), nil)
}

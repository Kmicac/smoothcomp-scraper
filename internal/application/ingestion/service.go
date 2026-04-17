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
	platformconfig "github.com/kmicac/smoothcomp-scraper/internal/platform/config"
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

type LifecycleObserver interface {
	OnClaim(context.Context, *job.Job)
	OnHeartbeat(context.Context, *job.Job)
	OnRetryScheduled(context.Context, *job.Job, time.Time)
	OnCompleted(context.Context, *job.Job)
	OnTerminalFailure(context.Context, *job.Job)
}

type noopLifecycleObserver struct{}

func (noopLifecycleObserver) OnClaim(context.Context, *job.Job)                     {}
func (noopLifecycleObserver) OnHeartbeat(context.Context, *job.Job)                 {}
func (noopLifecycleObserver) OnRetryScheduled(context.Context, *job.Job, time.Time) {}
func (noopLifecycleObserver) OnCompleted(context.Context, *job.Job)                 {}
func (noopLifecycleObserver) OnTerminalFailure(context.Context, *job.Job)           {}

type CommandService struct {
	jobs               port.JobRepository
	registry           *Registry
	logger             *zap.Logger
	defaultMaxAttempts int
}

func NewCommandService(jobs port.JobRepository, registry *Registry, logger *zap.Logger, defaultMaxAttempts int) *CommandService {
	return &CommandService{
		jobs:               jobs,
		registry:           registry,
		logger:             logger,
		defaultMaxAttempts: defaultMaxAttempts,
	}
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
		MaxAttempts:          s.defaultMaxAttempts,
		NextRetryAt:          &now,
		LastTransitionAt:     now,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	if err := s.jobs.Create(ctx, record); err != nil {
		return nil, coreerrors.New(coreerrors.CategoryStorage, coreerrors.CodeStorageFailed, "ingestion.enqueue", true, "failed to persist job", err)
	}

	s.logger.Info("job enqueued",
		zap.String("job_id", record.ID),
		zap.String("pipeline", string(record.Pipeline)),
		zap.String("correlation_id", record.CorrelationID),
		zap.Int("max_attempts", record.MaxAttempts),
	)

	return record, nil
}

type Worker struct {
	jobs      port.JobRepository
	snapshots port.SnapshotRepository
	results   port.ResultRepository
	registry  *Registry
	logger    *zap.Logger
	observer  LifecycleObserver
	workerID  string
	interval  time.Duration
	lease     time.Duration
	heartbeat time.Duration
	baseRetry time.Duration
	maxRetry  time.Duration
}

func NewWorker(
	jobs port.JobRepository,
	snapshots port.SnapshotRepository,
	results port.ResultRepository,
	registry *Registry,
	logger *zap.Logger,
	cfg platformconfig.WorkerConfig,
) *Worker {
	observer := LifecycleObserver(noopLifecycleObserver{})
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 5 * time.Second
	}
	return &Worker{
		jobs:      jobs,
		snapshots: snapshots,
		results:   results,
		registry:  registry,
		logger:    logger,
		observer:  observer,
		workerID:  job.NewID("worker"),
		interval:  cfg.PollInterval,
		lease:     cfg.LeaseDuration,
		heartbeat: cfg.HeartbeatInterval,
		baseRetry: cfg.BaseRetryDelay,
		maxRetry:  cfg.MaxRetryDelay,
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
	record, err := w.jobs.ClaimNextAvailable(ctx, job.ClaimOptions{
		WorkerID:      w.workerID,
		LeaseDuration: w.lease,
	})
	if err != nil {
		return false, coreerrors.New(coreerrors.CategoryStorage, coreerrors.CodeStorageFailed, "ingestion.worker.claim", true, "failed to claim job", err)
	}
	if record == nil {
		return false, nil
	}

	w.observer.OnClaim(ctx, record)
	w.logger.Info("job claimed",
		zap.String("job_id", record.ID),
		zap.String("pipeline", string(record.Pipeline)),
		zap.String("worker_id", w.workerID),
		zap.Int("attempt_count", record.AttemptCount),
		zap.Timep("lease_until", record.LeaseUntil),
	)

	pipeline, ok := w.registry.Get(record.Pipeline)
	if !ok {
		failErr := coreerrors.New(coreerrors.CategoryValidation, coreerrors.CodeInvalidRequest, "ingestion.worker.pipeline", false, "pipeline not registered", nil)
		return true, w.finishFailure(ctx, record, failErr)
	}

	heartbeatStop := w.startHeartbeat(ctx, record)
	defer close(heartbeatStop)

	startedAt := time.Now().UTC()
	snapshots, err := pipeline.Fetch(ctx, record.Request)
	if err != nil {
		return true, w.finishFailure(ctx, record, err)
	}

	for i := range snapshots {
		snapshots[i].JobID = record.ID
		snapshots[i].AttemptNumber = record.AttemptCount
		snapshots[i].Provider = record.Provider
		snapshots[i].Pipeline = record.Pipeline
		snapshots[i].ParserVersion = record.ParserVersion
		if err := w.snapshots.Save(ctx, &snapshots[i]); err != nil {
			failErr := coreerrors.New(coreerrors.CategoryStorage, coreerrors.CodeStorageFailed, "ingestion.worker.save_snapshot", true, "failed to store raw snapshot", err)
			return true, w.finishFailure(ctx, record, failErr)
		}
	}

	normalizedPayload, err := pipeline.Normalize(ctx, record.Request, snapshots)
	if err != nil {
		return true, w.finishFailure(ctx, record, err)
	}
	normalizedPayload.JobID = record.ID

	normalized := &job.NormalizedResult{
		ID:                   job.NewID("norm"),
		JobID:                record.ID,
		AttemptNumber:        record.AttemptCount,
		Provider:             record.Provider,
		Pipeline:             record.Pipeline,
		ParserVersion:        record.ParserVersion,
		NormalizationVersion: record.NormalizationVersion,
		CreatedAt:            time.Now().UTC(),
		Payload:              normalizedPayload,
	}
	if err := w.results.SaveNormalized(ctx, normalized); err != nil {
		failErr := coreerrors.New(coreerrors.CategoryStorage, coreerrors.CodeStorageFailed, "ingestion.worker.save_normalized", true, "failed to store normalized result", err)
		return true, w.finishFailure(ctx, record, failErr)
	}

	publishedPayload, err := pipeline.Publish(ctx, record.Request, normalizedPayload)
	if err != nil {
		return true, w.finishFailure(ctx, record, err)
	}
	publishedPayload.JobID = record.ID

	published := &job.PublishedResult{
		ID:              job.NewID("pub"),
		JobID:           record.ID,
		AttemptNumber:   record.AttemptCount,
		Provider:        record.Provider,
		Pipeline:        record.Pipeline,
		ContractVersion: publishedPayload.ContractVersion,
		PublishedAt:     time.Now().UTC(),
		Payload:         publishedPayload,
	}
	if err := w.results.SavePublished(ctx, published); err != nil {
		failErr := coreerrors.New(coreerrors.CategoryStorage, coreerrors.CodeStorageFailed, "ingestion.worker.save_published", true, "failed to store published result", err)
		return true, w.finishFailure(ctx, record, failErr)
	}

	completed, err := w.jobs.Complete(ctx, job.Completion{
		JobID:         record.ID,
		AttemptNumber: record.AttemptCount,
		WorkerID:      w.workerID,
		Stats:         StatsFromEnvelope(len(snapshots), publishedPayload),
		FinishedAt:    time.Now().UTC(),
	})
	if err != nil {
		return true, coreerrors.New(coreerrors.CategoryStorage, coreerrors.CodeStorageFailed, "ingestion.worker.complete", true, "failed to complete job", err)
	}

	w.observer.OnCompleted(ctx, completed)
	w.logger.Info("job completed",
		zap.String("job_id", completed.ID),
		zap.String("pipeline", string(completed.Pipeline)),
		zap.String("worker_id", w.workerID),
		zap.Int("attempt_count", completed.AttemptCount),
		zap.Int("snapshots", completed.Stats.SnapshotCount),
		zap.Int("events", completed.Stats.EventCount),
		zap.Int("people", completed.Stats.PersonCount),
		zap.Int("registrations", completed.Stats.RegistrationCount),
		zap.Int("matches", completed.Stats.MatchCount),
		zap.Duration("duration", time.Since(startedAt)),
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
		MatchCount:        len(payload.Matches),
	}
}

func (w *Worker) startHeartbeat(ctx context.Context, record *job.Job) chan struct{} {
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(w.heartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
				now := time.Now().UTC()
				if err := w.jobs.Heartbeat(ctx, job.LeaseHeartbeat{
					JobID:         record.ID,
					AttemptNumber: record.AttemptCount,
					WorkerID:      w.workerID,
					LeaseDuration: w.lease,
					HeartbeatAt:   now,
				}); err != nil {
					w.logger.Warn("failed to extend lease",
						zap.String("job_id", record.ID),
						zap.String("worker_id", w.workerID),
						zap.Error(err),
					)
					continue
				}
				record.LastHeartbeatAt = &now
				leaseUntil := now.Add(w.lease)
				record.LeaseUntil = &leaseUntil
				w.observer.OnHeartbeat(ctx, record)
			}
		}
	}()
	return stop
}

func (w *Worker) finishFailure(ctx context.Context, record *job.Job, err error) error {
	failure := failureFromError(err)
	transition := job.FailureTransition{
		JobID:         record.ID,
		AttemptNumber: record.AttemptCount,
		WorkerID:      w.workerID,
		Failure:       *failure,
		FinishedAt:    time.Now().UTC(),
	}

	if failure.Retryable && record.AttemptCount < record.MaxAttempts {
		retryAt := time.Now().UTC().Add(calculateRetryDelay(record.AttemptCount, w.baseRetry, w.maxRetry))
		transition.RetryAt = &retryAt
		record.NextRetryAt = &retryAt
	}
	if transition.RetryAt == nil {
		transition.Terminal = true
	}

	updated, persistErr := w.jobs.Fail(ctx, transition)
	if persistErr != nil {
		w.logger.Error("failed to persist job failure state", zap.Error(persistErr), zap.String("job_id", record.ID))
		return persistErr
	}

	if updated.State == job.StatePending && updated.NextRetryAt != nil {
		w.observer.OnRetryScheduled(ctx, updated, *updated.NextRetryAt)
		w.logger.Warn("job scheduled for retry",
			zap.String("job_id", updated.ID),
			zap.String("pipeline", string(updated.Pipeline)),
			zap.String("worker_id", w.workerID),
			zap.Int("attempt_count", updated.AttemptCount),
			zap.Time("next_retry_at", *updated.NextRetryAt),
			zap.String("error_code", failure.Code),
		)
		return err
	}

	w.observer.OnTerminalFailure(ctx, updated)
	w.logger.Error("job failed permanently",
		zap.String("job_id", updated.ID),
		zap.String("pipeline", string(updated.Pipeline)),
		zap.String("worker_id", w.workerID),
		zap.Int("attempt_count", updated.AttemptCount),
		zap.String("error_code", failure.Code),
		zap.Bool("retryable", failure.Retryable),
		zap.Error(err),
	)
	return err
}

func failureFromError(err error) *job.Failure {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &job.Failure{
			Category:  string(coreerrors.CategoryInternal),
			Code:      string(coreerrors.CodeInternal),
			Message:   err.Error(),
			Retryable: true,
		}
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

func calculateRetryDelay(attemptCount int, baseDelay time.Duration, maxDelay time.Duration) time.Duration {
	delay := baseDelay
	for i := 1; i < attemptCount; i++ {
		delay *= 2
		if delay >= maxDelay {
			return maxDelay
		}
	}
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

func UnsupportedPipelineError(name job.Pipeline) error {
	return coreerrors.New(coreerrors.CategoryValidation, coreerrors.CodeInvalidRequest, "ingestion.pipeline", false, fmt.Sprintf("pipeline %s is not supported", name), nil)
}

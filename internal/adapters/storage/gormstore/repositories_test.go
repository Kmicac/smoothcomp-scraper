package gormstore

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kmicac/smoothcomp-scraper/internal/core/contract"
	"github.com/kmicac/smoothcomp-scraper/internal/core/job"
	platformconfig "github.com/kmicac/smoothcomp-scraper/internal/platform/config"
)

func TestJobLifecycleRetryAndCompletion(t *testing.T) {
	ctx := context.Background()
	repos, cleanup := newTestRepositories(t)
	defer cleanup()

	now := time.Now().UTC()
	record := &job.Job{
		ID:                   "job_test_lifecycle",
		Provider:             "smoothcomp",
		Pipeline:             job.PipelineSmoothcompEventCatalog,
		Trigger:              job.TriggerManual,
		State:                job.StatePending,
		CorrelationID:        "corr_test",
		ParserVersion:        "parser.v1",
		NormalizationVersion: "norm.v1",
		Request: job.Request{
			Pipeline:      job.PipelineSmoothcompEventCatalog,
			Trigger:       job.TriggerManual,
			CorrelationID: "corr_test",
			Country:       "AR",
			EventType:     "past",
		},
		MaxAttempts:      3,
		NextRetryAt:      &now,
		LastTransitionAt: now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := repos.Create(ctx, record); err != nil {
		t.Fatalf("create job: %v", err)
	}

	claimed, err := repos.ClaimNextAvailable(ctx, job.ClaimOptions{
		WorkerID:      "worker_a",
		LeaseDuration: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("claim first attempt: %v", err)
	}
	if claimed == nil || claimed.AttemptCount != 1 {
		t.Fatalf("expected first claim with attempt 1, got %#v", claimed)
	}

	secondClaim, err := repos.ClaimNextAvailable(ctx, job.ClaimOptions{
		WorkerID:      "worker_b",
		LeaseDuration: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("claim during active lease: %v", err)
	}
	if secondClaim != nil {
		t.Fatalf("expected active lease to block second claim, got %#v", secondClaim)
	}

	retryAt := time.Now().UTC()
	failed, err := repos.Fail(ctx, job.FailureTransition{
		JobID:         claimed.ID,
		AttemptNumber: claimed.AttemptCount,
		WorkerID:      "worker_a",
		Failure: job.Failure{
			Category:  "external",
			Code:      "provider_failed",
			Message:   "transient outage",
			Retryable: true,
		},
		RetryAt:    &retryAt,
		FinishedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("schedule retry: %v", err)
	}
	if failed.State != job.StatePending || failed.NextRetryAt == nil {
		t.Fatalf("expected pending retry state, got %#v", failed)
	}

	reclaimed, err := repos.ClaimNextAvailable(ctx, job.ClaimOptions{
		WorkerID:      "worker_b",
		LeaseDuration: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("claim retry attempt: %v", err)
	}
	if reclaimed == nil || reclaimed.AttemptCount != 2 {
		t.Fatalf("expected second claim with attempt 2, got %#v", reclaimed)
	}

	completed, err := repos.Complete(ctx, job.Completion{
		JobID:         reclaimed.ID,
		AttemptNumber: reclaimed.AttemptCount,
		WorkerID:      "worker_b",
		Stats: job.Stats{
			SnapshotCount: 1,
			EventCount:    2,
		},
		FinishedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("complete job: %v", err)
	}
	if completed.State != job.StateSucceeded {
		t.Fatalf("expected succeeded state, got %s", completed.State)
	}
	if completed.AttemptCount != 2 {
		t.Fatalf("expected 2 attempts, got %d", completed.AttemptCount)
	}
}

func TestSaveResultsUpsertByJobID(t *testing.T) {
	ctx := context.Background()
	repos, cleanup := newTestRepositories(t)
	defer cleanup()

	payload := contract.Envelope{
		ContractVersion: contract.CurrentContractVersion,
		Provider:        "smoothcomp",
		Pipeline:        string(job.PipelineSmoothcompEventCatalog),
		GeneratedAt:     time.Now().UTC(),
	}

	first := &job.NormalizedResult{
		ID:                   "norm_1",
		JobID:                "job_results",
		AttemptNumber:        1,
		Provider:             "smoothcomp",
		Pipeline:             job.PipelineSmoothcompEventCatalog,
		ScopeKey:             "provider=smoothcomp|pipeline=smoothcomp.event_catalog|country=AR|event_type=past|event_id=|profile_id=",
		ParserVersion:        "parser.v1",
		NormalizationVersion: "norm.v1",
		ContractVersion:      contract.CurrentContractVersion,
		SourceSnapshotHash:   "source_hash_1",
		NormalizedHash:       "normalized_hash_1",
		PublicationDecision:  job.PublicationDecisionPublishChanged,
		PublicationReason:    "first_effective_publication",
		ChangeClassification: job.ChangeClassificationContentChanged,
		CreatedAt:            time.Now().UTC(),
		Payload:              payload,
	}
	if err := repos.SaveNormalized(ctx, first); err != nil {
		t.Fatalf("save normalized first: %v", err)
	}

	second := &job.NormalizedResult{
		ID:                   "norm_2",
		JobID:                "job_results",
		AttemptNumber:        2,
		Provider:             "smoothcomp",
		Pipeline:             job.PipelineSmoothcompEventCatalog,
		ScopeKey:             "provider=smoothcomp|pipeline=smoothcomp.event_catalog|country=AR|event_type=past|event_id=|profile_id=",
		ParserVersion:        "parser.v2",
		NormalizationVersion: "norm.v2",
		ContractVersion:      contract.CurrentContractVersion,
		SourceSnapshotHash:   "source_hash_2",
		NormalizedHash:       "normalized_hash_2",
		PublicationDecision:  job.PublicationDecisionPublishChanged,
		PublicationReason:    "source_snapshot_hash_changed",
		ChangeClassification: job.ChangeClassificationContentChanged,
		CreatedAt:            time.Now().UTC(),
		Payload:              payload,
	}
	if err := repos.SaveNormalized(ctx, second); err != nil {
		t.Fatalf("save normalized second: %v", err)
	}

	published := &job.PublishedResult{
		ID:                   "pub_1",
		JobID:                "job_results",
		AttemptNumber:        2,
		Provider:             "smoothcomp",
		Pipeline:             job.PipelineSmoothcompEventCatalog,
		ScopeKey:             "provider=smoothcomp|pipeline=smoothcomp.event_catalog|country=AR|event_type=past|event_id=|profile_id=",
		ParserVersion:        "parser.v2",
		NormalizationVersion: "norm.v2",
		ContractVersion:      contract.CurrentContractVersion,
		SourceSnapshotHash:   "source_hash_2",
		NormalizedHash:       "normalized_hash_2",
		PublicationDecision:  job.PublicationDecisionPublishChanged,
		PublicationReason:    "source_snapshot_hash_changed",
		ChangeClassification: job.ChangeClassificationContentChanged,
		PublishedAt:          time.Now().UTC(),
		Payload:              payload,
	}
	if err := repos.SavePublished(ctx, published); err != nil {
		t.Fatalf("save published: %v", err)
	}

	latest, err := repos.GetLatestPublished(ctx, job.PipelineSmoothcompEventCatalog)
	if err != nil {
		t.Fatalf("get latest published: %v", err)
	}
	if latest.JobID != "job_results" {
		t.Fatalf("unexpected published job id: %s", latest.JobID)
	}
	if latest.AttemptNumber != 2 {
		t.Fatalf("expected published attempt 2, got %d", latest.AttemptNumber)
	}
	if latest.EnvelopeHash == "" {
		t.Fatal("expected envelope hash to be populated")
	}

	latestByScope, err := repos.GetLatestPublishedByScope(ctx, job.PipelineSmoothcompEventCatalog, published.ScopeKey)
	if err != nil {
		t.Fatalf("get latest published by scope: %v", err)
	}
	if latestByScope.ID != published.ID {
		t.Fatalf("unexpected latest by scope id: %s", latestByScope.ID)
	}
}

func newTestRepositories(t *testing.T) (*Repositories, func()) {
	t.Helper()

	store, err := Open(platformconfig.StorageConfig{
		Driver:          "sqlite",
		DSN:             filepath.Join(t.TempDir(), "adapter.db"),
		RunMigrations:   true,
		MaxOpenConns:    1,
		MaxIdleConns:    1,
		ConnMaxLifetime: time.Minute,
	})
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}

	return NewRepositories(store), func() {
		_ = store.Close()
	}
}

package gormstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/kmicac/smoothcomp-scraper/internal/core/contract"
	"github.com/kmicac/smoothcomp-scraper/internal/core/job"
	"github.com/kmicac/smoothcomp-scraper/internal/core/port"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repositories struct {
	store *Store
}

func NewRepositories(store *Store) *Repositories {
	return &Repositories{store: store}
}

func (r *Repositories) Create(ctx context.Context, record *job.Job) error {
	requestJSON, err := marshal(record.Request)
	if err != nil {
		return err
	}
	statsJSON, err := marshal(record.Stats)
	if err != nil {
		return err
	}
	errorJSON, err := marshal(record.Error)
	if err != nil {
		return err
	}

	model := jobModel{
		ID:                   record.ID,
		Provider:             record.Provider,
		Pipeline:             string(record.Pipeline),
		Trigger:              string(record.Trigger),
		State:                string(record.State),
		CorrelationID:        record.CorrelationID,
		ParserVersion:        record.ParserVersion,
		NormalizationVersion: record.NormalizationVersion,
		Country:              record.Request.Country,
		EventType:            record.Request.EventType,
		EventID:              record.Request.EventID,
		EventURL:             record.Request.EventURL,
		EventName:            record.Request.EventName,
		RequestJSON:          requestJSON,
		StatsJSON:            statsJSON,
		ErrorJSON:            errorJSON,
		AttemptCount:         record.AttemptCount,
		MaxAttempts:          record.MaxAttempts,
		NextRetryAt:          record.NextRetryAt,
		LastTransitionAt:     record.LastTransitionAt,
		CreatedAt:            record.CreatedAt,
		UpdatedAt:            record.UpdatedAt,
	}
	if record.Error != nil {
		model.ErrorCategory = record.Error.Category
		model.ErrorCode = record.Error.Code
		model.ErrorMessage = record.Error.Message
		model.ErrorRetryable = record.Error.Retryable
	}

	return r.store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model).Error; err != nil {
			return err
		}
		return insertTransition(tx, record.ID, nil, "", string(record.State), "job_created", map[string]string{
			"pipeline": string(record.Pipeline),
		})
	})
}

func (r *Repositories) ClaimNextAvailable(ctx context.Context, options job.ClaimOptions) (*job.Job, error) {
	if options.WorkerID == "" {
		return nil, fmt.Errorf("worker id is required")
	}
	if options.LeaseDuration <= 0 {
		return nil, fmt.Errorf("lease duration must be greater than zero")
	}

	var claimed *job.Job
	err := r.store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		model, err := r.claimJobTx(tx, options)
		if err != nil {
			return err
		}
		if model == nil {
			return nil
		}

		if err := insertAttempt(tx, model); err != nil {
			return err
		}
		attemptNumber := model.AttemptCount
		if err := insertTransition(tx, model.ID, &attemptNumber, "", model.State, "job_claimed", map[string]string{
			"claimed_by": options.WorkerID,
		}); err != nil {
			return err
		}

		record, err := toCoreJob(*model)
		if err != nil {
			return err
		}
		claimed = record
		return nil
	})
	return claimed, err
}

func (r *Repositories) Heartbeat(ctx context.Context, heartbeat job.LeaseHeartbeat) error {
	leaseUntil := heartbeat.HeartbeatAt.Add(heartbeat.LeaseDuration)
	return r.store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&jobModel{}).
			Where("id = ? AND state = ? AND claimed_by = ? AND attempt_count = ?", heartbeat.JobID, string(job.StateRunning), heartbeat.WorkerID, heartbeat.AttemptNumber).
			Updates(map[string]any{
				"lease_until":       leaseUntil,
				"last_heartbeat_at": heartbeat.HeartbeatAt,
				"updated_at":        heartbeat.HeartbeatAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("job %s is no longer leased by worker %s", heartbeat.JobID, heartbeat.WorkerID)
		}

		return tx.Model(&jobAttemptModel{}).
			Where("job_id = ? AND attempt_number = ?", heartbeat.JobID, heartbeat.AttemptNumber).
			Updates(map[string]any{
				"lease_until":       leaseUntil,
				"last_heartbeat_at": heartbeat.HeartbeatAt,
			}).Error
	})
}

func (r *Repositories) Complete(ctx context.Context, completion job.Completion) (*job.Job, error) {
	var updated *job.Job
	err := r.store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		statsJSON, err := marshal(completion.Stats)
		if err != nil {
			return err
		}

		result := tx.Model(&jobModel{}).
			Where("id = ? AND state = ? AND claimed_by = ? AND attempt_count = ?", completion.JobID, string(job.StateRunning), completion.WorkerID, completion.AttemptNumber).
			Updates(map[string]any{
				"state":              string(job.StateSucceeded),
				"stats_json":         statsJSON,
				"claimed_by":         nil,
				"claimed_at":         nil,
				"lease_until":        nil,
				"last_heartbeat_at":  nil,
				"next_retry_at":      nil,
				"error_json":         nil,
				"error_category":     "",
				"error_code":         "",
				"error_message":      "",
				"error_retryable":    false,
				"finished_at":        completion.FinishedAt,
				"updated_at":         completion.FinishedAt,
				"last_transition_at": completion.FinishedAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("job %s completion update did not match an active lease", completion.JobID)
		}

		if err := tx.Model(&jobAttemptModel{}).
			Where("job_id = ? AND attempt_number = ?", completion.JobID, completion.AttemptNumber).
			Updates(map[string]any{
				"state":             string(job.StateSucceeded),
				"finished_at":       completion.FinishedAt,
				"lease_until":       nil,
				"last_heartbeat_at": completion.FinishedAt,
			}).Error; err != nil {
			return err
		}

		if err := insertTransition(tx, completion.JobID, &completion.AttemptNumber, string(job.StateRunning), string(job.StateSucceeded), "job_completed", nil); err != nil {
			return err
		}

		record, err := r.getJobTx(tx, completion.JobID)
		if err != nil {
			return err
		}
		updated = record
		return nil
	})
	return updated, err
}

func (r *Repositories) Fail(ctx context.Context, transition job.FailureTransition) (*job.Job, error) {
	var updated *job.Job
	err := r.store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		errorJSON, err := marshal(transition.Failure)
		if err != nil {
			return err
		}

		nextState := string(job.StateFailed)
		reason := "job_failed"
		updates := map[string]any{
			"error_json":         errorJSON,
			"error_category":     transition.Failure.Category,
			"error_code":         transition.Failure.Code,
			"error_message":      transition.Failure.Message,
			"error_retryable":    transition.Failure.Retryable,
			"claimed_by":         nil,
			"claimed_at":         nil,
			"lease_until":        nil,
			"last_heartbeat_at":  nil,
			"finished_at":        nil,
			"updated_at":         transition.FinishedAt,
			"last_transition_at": transition.FinishedAt,
		}
		if transition.RetryAt != nil {
			nextState = string(job.StatePending)
			reason = "job_retry_scheduled"
			updates["next_retry_at"] = *transition.RetryAt
		} else {
			updates["finished_at"] = transition.FinishedAt
			updates["next_retry_at"] = nil
			if transition.Terminal && transition.Failure.Retryable {
				nextState = string(job.StateExhausted)
				reason = "job_exhausted"
			}
		}
		updates["state"] = nextState

		result := tx.Model(&jobModel{}).
			Where("id = ? AND state = ? AND claimed_by = ? AND attempt_count = ?", transition.JobID, string(job.StateRunning), transition.WorkerID, transition.AttemptNumber).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("job %s failure update did not match an active lease", transition.JobID)
		}

		if err := tx.Model(&jobAttemptModel{}).
			Where("job_id = ? AND attempt_number = ?", transition.JobID, transition.AttemptNumber).
			Updates(map[string]any{
				"state":             string(job.StateFailed),
				"error_json":        errorJSON,
				"error_category":    transition.Failure.Category,
				"error_code":        transition.Failure.Code,
				"error_message":     transition.Failure.Message,
				"error_retryable":   transition.Failure.Retryable,
				"finished_at":       transition.FinishedAt,
				"lease_until":       nil,
				"last_heartbeat_at": transition.FinishedAt,
			}).Error; err != nil {
			return err
		}

		metadata := map[string]string{}
		if transition.RetryAt != nil {
			metadata["next_retry_at"] = transition.RetryAt.UTC().Format(time.RFC3339Nano)
		}
		if err := insertTransition(tx, transition.JobID, &transition.AttemptNumber, string(job.StateRunning), nextState, reason, metadata); err != nil {
			return err
		}

		record, err := r.getJobTx(tx, transition.JobID)
		if err != nil {
			return err
		}
		updated = record
		return nil
	})
	return updated, err
}

func (r *Repositories) Get(ctx context.Context, id string) (*job.Job, error) {
	return r.getJobTx(r.store.db.WithContext(ctx), id)
}

func (r *Repositories) List(ctx context.Context, filter port.JobListFilter) ([]job.Job, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var models []jobModel
	if err := r.store.db.WithContext(ctx).Order("created_at DESC").Limit(limit).Find(&models).Error; err != nil {
		return nil, err
	}

	records := make([]job.Job, 0, len(models))
	for _, model := range models {
		record, err := toCoreJob(model)
		if err != nil {
			return nil, err
		}
		records = append(records, *record)
	}
	return records, nil
}

func (r *Repositories) Save(ctx context.Context, snapshot *job.RawSnapshot) error {
	metadataJSON, err := encodeStringMap(snapshot.Metadata)
	if err != nil {
		return err
	}

	model := rawSnapshotModel{
		ID:            snapshot.ID,
		JobID:         snapshot.JobID,
		AttemptNumber: snapshot.AttemptNumber,
		Provider:      snapshot.Provider,
		Pipeline:      string(snapshot.Pipeline),
		ResourceType:  snapshot.ResourceType,
		ResourceKey:   snapshot.ResourceKey,
		SourceURL:     snapshot.SourceURL,
		ContentType:   snapshot.ContentType,
		StatusCode:    snapshot.StatusCode,
		ParserVersion: snapshot.ParserVersion,
		CapturedAt:    snapshot.CapturedAt,
		SHA256:        snapshot.SHA256,
		Body:          snapshot.Body,
		MetadataJSON:  metadataJSON,
	}

	return r.store.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "job_id"},
			{Name: "attempt_number"},
			{Name: "resource_type"},
			{Name: "resource_key"},
			{Name: "sha256"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"content_type":   model.ContentType,
			"status_code":    model.StatusCode,
			"captured_at":    model.CapturedAt,
			"metadata_json":  model.MetadataJSON,
			"body":           model.Body,
			"source_url":     model.SourceURL,
			"parser_version": model.ParserVersion,
		}),
	}).Create(&model).Error
}

func (r *Repositories) ListByJob(ctx context.Context, jobID string) ([]job.RawSnapshot, error) {
	var models []rawSnapshotModel
	if err := r.store.db.WithContext(ctx).Where("job_id = ?", jobID).Order("captured_at ASC").Find(&models).Error; err != nil {
		return nil, err
	}

	items := make([]job.RawSnapshot, 0, len(models))
	for _, model := range models {
		metadata, err := decodeStringMap(model.MetadataJSON)
		if err != nil {
			return nil, err
		}
		items = append(items, job.RawSnapshot{
			ID:            model.ID,
			JobID:         model.JobID,
			AttemptNumber: model.AttemptNumber,
			Provider:      model.Provider,
			Pipeline:      job.Pipeline(model.Pipeline),
			ResourceType:  model.ResourceType,
			ResourceKey:   model.ResourceKey,
			SourceURL:     model.SourceURL,
			ContentType:   model.ContentType,
			StatusCode:    model.StatusCode,
			ParserVersion: model.ParserVersion,
			CapturedAt:    model.CapturedAt,
			SHA256:        model.SHA256,
			Body:          model.Body,
			Metadata:      metadata,
		})
	}
	return items, nil
}

func (r *Repositories) SaveNormalized(ctx context.Context, result *job.NormalizedResult) error {
	payloadJSON, err := marshal(result.Payload)
	if err != nil {
		return err
	}
	metadataJSON, err := encodeStringMap(result.Metadata)
	if err != nil {
		return err
	}
	if result.NormalizedHash == "" {
		sum := sha256.Sum256(payloadJSON)
		result.NormalizedHash = hex.EncodeToString(sum[:])
	}

	model := normalizedResultModel{
		ID:                   result.ID,
		JobID:                result.JobID,
		AttemptNumber:        result.AttemptNumber,
		Provider:             result.Provider,
		Pipeline:             string(result.Pipeline),
		ScopeKey:             result.ScopeKey,
		ParserVersion:        result.ParserVersion,
		NormalizationVersion: result.NormalizationVersion,
		ContractVersion:      result.ContractVersion,
		SourceSnapshotHash:   result.SourceSnapshotHash,
		PayloadHash:          result.NormalizedHash,
		PublicationDecision:  string(result.PublicationDecision),
		PublicationReason:    result.PublicationReason,
		ChangeClassification: string(result.ChangeClassification),
		ForcedRepublish:      result.ForcedRepublish,
		CreatedAt:            result.CreatedAt,
		MetadataJSON:         metadataJSON,
		PayloadJSON:          payloadJSON,
	}

	return r.store.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "job_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"attempt_number":        model.AttemptNumber,
			"provider":              model.Provider,
			"pipeline":              model.Pipeline,
			"scope_key":             model.ScopeKey,
			"parser_version":        model.ParserVersion,
			"normalization_version": model.NormalizationVersion,
			"contract_version":      model.ContractVersion,
			"source_snapshot_hash":  model.SourceSnapshotHash,
			"payload_hash":          model.PayloadHash,
			"publication_decision":  model.PublicationDecision,
			"publication_reason":    model.PublicationReason,
			"change_classification": model.ChangeClassification,
			"forced_republish":      model.ForcedRepublish,
			"created_at":            model.CreatedAt,
			"metadata_json":         model.MetadataJSON,
			"payload_json":          model.PayloadJSON,
		}),
	}).Create(&model).Error
}

func (r *Repositories) SavePublished(ctx context.Context, result *job.PublishedResult) error {
	payloadJSON, err := marshal(result.Payload)
	if err != nil {
		return err
	}
	metadataJSON, err := encodeStringMap(result.Metadata)
	if err != nil {
		return err
	}
	if result.EnvelopeHash == "" {
		sum := sha256.Sum256(payloadJSON)
		result.EnvelopeHash = hex.EncodeToString(sum[:])
	}
	model := publishedResultModel{
		ID:                      result.ID,
		JobID:                   result.JobID,
		AttemptNumber:           result.AttemptNumber,
		Provider:                result.Provider,
		Pipeline:                string(result.Pipeline),
		ScopeKey:                result.ScopeKey,
		CorrelationID:           result.CorrelationID,
		ParserVersion:           result.ParserVersion,
		NormalizationVersion:    result.NormalizationVersion,
		ContractVersion:         result.ContractVersion,
		SourceSnapshotHash:      result.SourceSnapshotHash,
		NormalizedHash:          result.NormalizedHash,
		Checksum:                result.EnvelopeHash,
		PublicationDecision:     string(result.PublicationDecision),
		PublicationReason:       result.PublicationReason,
		ChangeClassification:    string(result.ChangeClassification),
		ForcedRepublish:         result.ForcedRepublish,
		SupersedesPublicationID: result.SupersedesPublicationID,
		PublishedAt:             result.PublishedAt,
		MetadataJSON:            metadataJSON,
		PayloadJSON:             payloadJSON,
	}
	result.EnvelopeHash = model.Checksum

	return r.store.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "job_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"attempt_number":            model.AttemptNumber,
			"provider":                  model.Provider,
			"pipeline":                  model.Pipeline,
			"scope_key":                 model.ScopeKey,
			"correlation_id":            model.CorrelationID,
			"parser_version":            model.ParserVersion,
			"normalization_version":     model.NormalizationVersion,
			"contract_version":          model.ContractVersion,
			"source_snapshot_hash":      model.SourceSnapshotHash,
			"normalized_hash":           model.NormalizedHash,
			"checksum":                  model.Checksum,
			"publication_decision":      model.PublicationDecision,
			"publication_reason":        model.PublicationReason,
			"change_classification":     model.ChangeClassification,
			"forced_republish":          model.ForcedRepublish,
			"supersedes_publication_id": model.SupersedesPublicationID,
			"published_at":              model.PublishedAt,
			"metadata_json":             model.MetadataJSON,
			"payload_json":              model.PayloadJSON,
		}),
	}).Create(&model).Error
}

func (r *Repositories) GetLatestPublished(ctx context.Context, pipeline job.Pipeline) (*job.PublishedResult, error) {
	var model publishedResultModel
	if err := r.store.db.WithContext(ctx).
		Where("pipeline = ?", string(pipeline)).
		Order("published_at DESC").
		First(&model).Error; err != nil {
		return nil, err
	}

	var payload contract.Envelope
	if err := unmarshal(model.PayloadJSON, &payload); err != nil {
		return nil, err
	}
	metadata, err := decodeStringMap(model.MetadataJSON)
	if err != nil {
		return nil, err
	}
	return &job.PublishedResult{
		ID:                      model.ID,
		JobID:                   model.JobID,
		AttemptNumber:           model.AttemptNumber,
		Provider:                model.Provider,
		Pipeline:                job.Pipeline(model.Pipeline),
		ScopeKey:                model.ScopeKey,
		CorrelationID:           model.CorrelationID,
		ParserVersion:           model.ParserVersion,
		NormalizationVersion:    model.NormalizationVersion,
		ContractVersion:         model.ContractVersion,
		SourceSnapshotHash:      model.SourceSnapshotHash,
		NormalizedHash:          model.NormalizedHash,
		EnvelopeHash:            model.Checksum,
		PublicationDecision:     job.PublicationDecision(model.PublicationDecision),
		PublicationReason:       model.PublicationReason,
		ChangeClassification:    job.ChangeClassification(model.ChangeClassification),
		ForcedRepublish:         model.ForcedRepublish,
		SupersedesPublicationID: model.SupersedesPublicationID,
		PublishedAt:             model.PublishedAt,
		Metadata:                metadata,
		Payload:                 payload,
	}, nil
}

func (r *Repositories) GetLatestPublishedByScope(ctx context.Context, pipeline job.Pipeline, scopeKey string) (*job.PublishedResult, error) {
	var model publishedResultModel
	if err := r.store.db.WithContext(ctx).
		Where("pipeline = ? AND scope_key = ?", string(pipeline), scopeKey).
		Order("published_at DESC").
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, port.ErrPublicationNotFound
		}
		return nil, err
	}

	var payload contract.Envelope
	if err := unmarshal(model.PayloadJSON, &payload); err != nil {
		return nil, err
	}
	metadata, err := decodeStringMap(model.MetadataJSON)
	if err != nil {
		return nil, err
	}
	return &job.PublishedResult{
		ID:                      model.ID,
		JobID:                   model.JobID,
		AttemptNumber:           model.AttemptNumber,
		Provider:                model.Provider,
		Pipeline:                job.Pipeline(model.Pipeline),
		ScopeKey:                model.ScopeKey,
		CorrelationID:           model.CorrelationID,
		ParserVersion:           model.ParserVersion,
		NormalizationVersion:    model.NormalizationVersion,
		ContractVersion:         model.ContractVersion,
		SourceSnapshotHash:      model.SourceSnapshotHash,
		NormalizedHash:          model.NormalizedHash,
		EnvelopeHash:            model.Checksum,
		PublicationDecision:     job.PublicationDecision(model.PublicationDecision),
		PublicationReason:       model.PublicationReason,
		ChangeClassification:    job.ChangeClassification(model.ChangeClassification),
		ForcedRepublish:         model.ForcedRepublish,
		SupersedesPublicationID: model.SupersedesPublicationID,
		PublishedAt:             model.PublishedAt,
		Metadata:                metadata,
		Payload:                 payload,
	}, nil
}

func (r *Repositories) Upsert(ctx context.Context, schedule *job.Schedule) error {
	model := scheduleModel{
		Name:           schedule.Name,
		CronExpression: schedule.CronExpression,
		Enabled:        schedule.Enabled,
		UpdatedAt:      time.Now().UTC(),
	}
	return r.store.db.WithContext(ctx).Save(&model).Error
}

func (r *Repositories) Ping(ctx context.Context) error {
	return r.store.Ping(ctx)
}

func (r *Repositories) claimJobTx(tx *gorm.DB, options job.ClaimOptions) (*jobModel, error) {
	switch r.store.dialect {
	case "postgres":
		return r.claimJobPostgres(tx, options)
	default:
		return r.claimJobSQLite(tx, options)
	}
}

func (r *Repositories) claimJobPostgres(tx *gorm.DB, options job.ClaimOptions) (*jobModel, error) {
	type row struct {
		jobModel
	}
	var result row
	query := `
WITH candidate AS (
	SELECT id
	FROM ingestion_jobs
	WHERE (
		(state = 'pending' AND COALESCE(next_retry_at, created_at) <= NOW())
		OR (state = 'running' AND lease_until IS NOT NULL AND lease_until < NOW())
	)
	ORDER BY COALESCE(next_retry_at, created_at) ASC, created_at ASC
	FOR UPDATE SKIP LOCKED
	LIMIT 1
)
UPDATE ingestion_jobs AS jobs
SET state = $1,
	claimed_by = $2,
	claimed_at = NOW(),
	lease_until = NOW() + ($3 * INTERVAL '1 second'),
	last_heartbeat_at = NOW(),
	next_retry_at = NULL,
	attempt_count = jobs.attempt_count + 1,
	started_at = COALESCE(jobs.started_at, NOW()),
	finished_at = NULL,
	updated_at = NOW(),
	last_transition_at = NOW()
FROM candidate
WHERE jobs.id = candidate.id
RETURNING jobs.*`

	if err := tx.Raw(query, string(job.StateRunning), options.WorkerID, int(options.LeaseDuration.Seconds())).Scan(&result).Error; err != nil {
		return nil, err
	}
	if result.ID == "" {
		return nil, nil
	}
	return &result.jobModel, nil
}

func (r *Repositories) claimJobSQLite(tx *gorm.DB, options job.ClaimOptions) (*jobModel, error) {
	now := time.Now().UTC()
	var model jobModel
	if err := tx.
		Where("(state = ? AND COALESCE(next_retry_at, created_at) <= ?) OR (state = ? AND lease_until IS NOT NULL AND lease_until < ?)", string(job.StatePending), now, string(job.StateRunning), now).
		Order("COALESCE(next_retry_at, created_at) ASC").
		Order("created_at ASC").
		First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	leaseUntil := now.Add(options.LeaseDuration)
	result := tx.Model(&jobModel{}).
		Where("id = ? AND (((state = ? AND COALESCE(next_retry_at, created_at) <= ?)) OR (state = ? AND lease_until IS NOT NULL AND lease_until < ?))", model.ID, string(job.StatePending), now, string(job.StateRunning), now).
		Updates(map[string]any{
			"state":              string(job.StateRunning),
			"claimed_by":         options.WorkerID,
			"claimed_at":         now,
			"lease_until":        leaseUntil,
			"last_heartbeat_at":  now,
			"next_retry_at":      nil,
			"attempt_count":      model.AttemptCount + 1,
			"started_at":         coalesceTime(model.StartedAt, now),
			"finished_at":        nil,
			"updated_at":         now,
			"last_transition_at": now,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}

	model.State = string(job.StateRunning)
	model.ClaimedBy = options.WorkerID
	model.ClaimedAt = &now
	model.LeaseUntil = &leaseUntil
	model.LastHeartbeatAt = &now
	model.NextRetryAt = nil
	model.AttemptCount++
	if model.StartedAt == nil {
		model.StartedAt = &now
	}
	model.FinishedAt = nil
	model.UpdatedAt = now
	model.LastTransitionAt = now
	return &model, nil
}

func (r *Repositories) getJobTx(tx *gorm.DB, id string) (*job.Job, error) {
	var model jobModel
	if err := tx.First(&model, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return toCoreJob(model)
}

func insertAttempt(tx *gorm.DB, model *jobModel) error {
	failure := failureFromModel(model)
	errorJSON, err := marshal(failure)
	if err != nil {
		return err
	}
	attempt := jobAttemptModel{
		ID:              job.NewID("attempt"),
		JobID:           model.ID,
		AttemptNumber:   model.AttemptCount,
		WorkerID:        model.ClaimedBy,
		State:           string(job.StateRunning),
		ErrorJSON:       errorJSON,
		ErrorCategory:   model.ErrorCategory,
		ErrorCode:       model.ErrorCode,
		ErrorMessage:    model.ErrorMessage,
		ErrorRetryable:  model.ErrorRetryable,
		StartedAt:       derefTime(model.ClaimedAt, time.Now().UTC()),
		LastHeartbeatAt: model.LastHeartbeatAt,
		LeaseUntil:      model.LeaseUntil,
		CreatedAt:       time.Now().UTC(),
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "job_id"}, {Name: "attempt_number"}},
		DoNothing: true,
	}).Create(&attempt).Error
}

func insertTransition(tx *gorm.DB, jobID string, attemptNumber *int, fromState string, toState string, reason string, metadata map[string]string) error {
	metadataJSON, err := encodeStringMap(metadata)
	if err != nil {
		return err
	}
	transition := jobTransitionModel{
		ID:            job.NewID("transition"),
		JobID:         jobID,
		AttemptNumber: attemptNumber,
		FromState:     fromState,
		ToState:       toState,
		Reason:        reason,
		MetadataJSON:  metadataJSON,
		CreatedAt:     time.Now().UTC(),
	}
	return tx.Create(&transition).Error
}

func toCoreJob(model jobModel) (*job.Job, error) {
	var request job.Request
	if err := unmarshal(model.RequestJSON, &request); err != nil {
		return nil, fmt.Errorf("decode request: %w", err)
	}
	var stats job.Stats
	if err := unmarshal(model.StatsJSON, &stats); err != nil {
		return nil, fmt.Errorf("decode stats: %w", err)
	}
	failure := failureFromModel(&model)
	return &job.Job{
		ID:                   model.ID,
		Provider:             model.Provider,
		Pipeline:             job.Pipeline(model.Pipeline),
		Trigger:              job.Trigger(model.Trigger),
		State:                job.State(model.State),
		CorrelationID:        model.CorrelationID,
		ParserVersion:        model.ParserVersion,
		NormalizationVersion: model.NormalizationVersion,
		Request:              request,
		Stats:                stats,
		Error:                failure,
		AttemptCount:         model.AttemptCount,
		MaxAttempts:          model.MaxAttempts,
		ClaimedBy:            model.ClaimedBy,
		ClaimedAt:            model.ClaimedAt,
		LeaseUntil:           model.LeaseUntil,
		LastHeartbeatAt:      model.LastHeartbeatAt,
		NextRetryAt:          model.NextRetryAt,
		LastTransitionAt:     model.LastTransitionAt,
		CreatedAt:            model.CreatedAt,
		UpdatedAt:            model.UpdatedAt,
		StartedAt:            model.StartedAt,
		FinishedAt:           model.FinishedAt,
	}, nil
}

func failureFromModel(model *jobModel) *job.Failure {
	if model == nil || (model.ErrorCategory == "" && model.ErrorCode == "" && model.ErrorMessage == "") {
		return nil
	}
	return &job.Failure{
		Category:  model.ErrorCategory,
		Code:      model.ErrorCode,
		Message:   model.ErrorMessage,
		Retryable: model.ErrorRetryable,
	}
}

func derefTime(value *time.Time, fallback time.Time) time.Time {
	if value == nil {
		return fallback
	}
	return *value
}

func coalesceTime(value *time.Time, fallback time.Time) time.Time {
	if value == nil {
		return fallback
	}
	return *value
}

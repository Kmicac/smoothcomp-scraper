package job

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/kmicac/smoothcomp-scraper/internal/core/contract"
)

type State string
type Trigger string
type Pipeline string
type PublicationDecision string
type ChangeClassification string

const (
	StatePending   State = "pending"
	StateRunning   State = "running"
	StateSucceeded State = "succeeded"
	StateFailed    State = "failed"
	StateExhausted State = "exhausted"

	TriggerManual    Trigger = "manual"
	TriggerScheduled Trigger = "scheduled"
	TriggerReplay    Trigger = "replay"

	PipelineSmoothcompEventCatalog      Pipeline = "smoothcomp.event_catalog"
	PipelineSmoothcompEventParticipants Pipeline = "smoothcomp.event_participants"
	PipelineSmoothcompEventDetail       Pipeline = "smoothcomp.event_detail"
	PipelineSmoothcompAthleteProfile    Pipeline = "smoothcomp.athlete_profile_enrichment"
	PipelineSmoothcompAcademyCatalog    Pipeline = "smoothcomp.academy_catalog"

	PublicationDecisionSkipNoChange   PublicationDecision = "SKIP_NO_CHANGE"
	PublicationDecisionPublishChanged PublicationDecision = "PUBLISH_CHANGED"
	PublicationDecisionPublishForced  PublicationDecision = "PUBLISH_FORCED"

	ChangeClassificationNoChange             ChangeClassification = "NO_CHANGE"
	ChangeClassificationContentChanged       ChangeClassification = "CONTENT_CHANGED"
	ChangeClassificationNormalizationChanged ChangeClassification = "NORMALIZATION_CHANGED"
	ChangeClassificationRepublishForced      ChangeClassification = "REPUBLISH_FORCED"
)

type Request struct {
	Pipeline      Pipeline          `json:"pipeline"`
	Trigger       Trigger           `json:"trigger"`
	CorrelationID string            `json:"correlation_id"`
	Country       string            `json:"country,omitempty"`
	EventType     string            `json:"event_type,omitempty"`
	EventID       string            `json:"event_id,omitempty"`
	EventURL      string            `json:"event_url,omitempty"`
	EventName     string            `json:"event_name,omitempty"`
	ProfileID     string            `json:"profile_id,omitempty"`
	ProfileURL    string            `json:"profile_url,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type Stats struct {
	SnapshotCount     int `json:"snapshot_count"`
	EventCount        int `json:"event_count"`
	OrganizationCount int `json:"organization_count"`
	PersonCount       int `json:"person_count"`
	RegistrationCount int `json:"registration_count"`
	MatchCount        int `json:"match_count"`
}

type Failure struct {
	Category  string `json:"category"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type Job struct {
	ID                   string     `json:"id"`
	Provider             string     `json:"provider"`
	Pipeline             Pipeline   `json:"pipeline"`
	Trigger              Trigger    `json:"trigger"`
	State                State      `json:"state"`
	CorrelationID        string     `json:"correlation_id"`
	ParserVersion        string     `json:"parser_version"`
	NormalizationVersion string     `json:"normalization_version"`
	Request              Request    `json:"request"`
	Stats                Stats      `json:"stats"`
	Error                *Failure   `json:"error,omitempty"`
	AttemptCount         int        `json:"attempt_count"`
	MaxAttempts          int        `json:"max_attempts"`
	ClaimedBy            string     `json:"claimed_by,omitempty"`
	ClaimedAt            *time.Time `json:"claimed_at,omitempty"`
	LeaseUntil           *time.Time `json:"lease_until,omitempty"`
	LastHeartbeatAt      *time.Time `json:"last_heartbeat_at,omitempty"`
	NextRetryAt          *time.Time `json:"next_retry_at,omitempty"`
	LastTransitionAt     time.Time  `json:"last_transition_at"`
	CreatedAt            time.Time  `json:"created_at"`
	StartedAt            *time.Time `json:"started_at,omitempty"`
	FinishedAt           *time.Time `json:"finished_at,omitempty"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type RawSnapshot struct {
	ID            string            `json:"id"`
	JobID         string            `json:"job_id"`
	AttemptNumber int               `json:"attempt_number"`
	Provider      string            `json:"provider"`
	Pipeline      Pipeline          `json:"pipeline"`
	ResourceType  string            `json:"resource_type"`
	ResourceKey   string            `json:"resource_key"`
	SourceURL     string            `json:"source_url"`
	ContentType   string            `json:"content_type"`
	StatusCode    int               `json:"status_code"`
	ParserVersion string            `json:"parser_version"`
	CapturedAt    time.Time         `json:"captured_at"`
	SHA256        string            `json:"sha256"`
	Body          []byte            `json:"body"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type NormalizedResult struct {
	ID                   string               `json:"id"`
	JobID                string               `json:"job_id"`
	AttemptNumber        int                  `json:"attempt_number"`
	Provider             string               `json:"provider"`
	Pipeline             Pipeline             `json:"pipeline"`
	ScopeKey             string               `json:"scope_key"`
	ParserVersion        string               `json:"parser_version"`
	NormalizationVersion string               `json:"normalization_version"`
	ContractVersion      string               `json:"contract_version"`
	SourceSnapshotHash   string               `json:"source_snapshot_hash"`
	NormalizedHash       string               `json:"normalized_hash"`
	PublicationDecision  PublicationDecision  `json:"publication_decision"`
	PublicationReason    string               `json:"publication_reason"`
	ChangeClassification ChangeClassification `json:"change_classification"`
	ForcedRepublish      bool                 `json:"forced_republish"`
	CreatedAt            time.Time            `json:"created_at"`
	Metadata             map[string]string    `json:"metadata,omitempty"`
	Payload              contract.Envelope    `json:"payload"`
}

type PublishedResult struct {
	ID                      string               `json:"id"`
	JobID                   string               `json:"job_id"`
	AttemptNumber           int                  `json:"attempt_number"`
	Provider                string               `json:"provider"`
	Pipeline                Pipeline             `json:"pipeline"`
	ScopeKey                string               `json:"scope_key"`
	CorrelationID           string               `json:"correlation_id,omitempty"`
	ParserVersion           string               `json:"parser_version"`
	NormalizationVersion    string               `json:"normalization_version"`
	ContractVersion         string               `json:"contract_version"`
	SourceSnapshotHash      string               `json:"source_snapshot_hash"`
	NormalizedHash          string               `json:"normalized_hash"`
	EnvelopeHash            string               `json:"envelope_hash"`
	PublicationDecision     PublicationDecision  `json:"publication_decision"`
	PublicationReason       string               `json:"publication_reason"`
	ChangeClassification    ChangeClassification `json:"change_classification"`
	ForcedRepublish         bool                 `json:"forced_republish"`
	SupersedesPublicationID string               `json:"supersedes_publication_id,omitempty"`
	PublishedAt             time.Time            `json:"published_at"`
	Metadata                map[string]string    `json:"metadata,omitempty"`
	Payload                 contract.Envelope    `json:"payload"`
}

type Schedule struct {
	Name           string    `json:"name"`
	CronExpression string    `json:"cron_expression"`
	Enabled        bool      `json:"enabled"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Attempt struct {
	ID              string     `json:"id"`
	JobID           string     `json:"job_id"`
	AttemptNumber   int        `json:"attempt_number"`
	WorkerID        string     `json:"worker_id"`
	State           State      `json:"state"`
	Error           *Failure   `json:"error,omitempty"`
	StartedAt       time.Time  `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at,omitempty"`
	LeaseUntil      *time.Time `json:"lease_until,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type ClaimOptions struct {
	WorkerID      string
	LeaseDuration time.Duration
}

type LeaseHeartbeat struct {
	JobID         string
	AttemptNumber int
	WorkerID      string
	LeaseDuration time.Duration
	HeartbeatAt   time.Time
}

type Completion struct {
	JobID         string
	AttemptNumber int
	WorkerID      string
	Stats         Stats
	FinishedAt    time.Time
}

type FailureTransition struct {
	JobID         string
	AttemptNumber int
	WorkerID      string
	Failure       Failure
	RetryAt       *time.Time
	FinishedAt    time.Time
	Terminal      bool
}

func (j Job) IsTerminal() bool {
	return j.State == StateSucceeded || j.State == StateFailed || j.State == StateExhausted
}

func NewID(prefix string) string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return prefix + "_" + time.Now().UTC().Format("20060102T150405.000000000") + "_" + hex.EncodeToString(buf)
}

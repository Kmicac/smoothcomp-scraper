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

const (
	StatePending   State = "pending"
	StateRunning   State = "running"
	StateSucceeded State = "succeeded"
	StateFailed    State = "failed"

	TriggerManual    Trigger = "manual"
	TriggerScheduled Trigger = "scheduled"
	TriggerReplay    Trigger = "replay"

	PipelineSmoothcompEventCatalog      Pipeline = "smoothcomp.event_catalog"
	PipelineSmoothcompEventParticipants Pipeline = "smoothcomp.event_participants"
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
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type Stats struct {
	SnapshotCount     int `json:"snapshot_count"`
	EventCount        int `json:"event_count"`
	OrganizationCount int `json:"organization_count"`
	PersonCount       int `json:"person_count"`
	RegistrationCount int `json:"registration_count"`
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
	WorkerID             string     `json:"worker_id,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	StartedAt            *time.Time `json:"started_at,omitempty"`
	FinishedAt           *time.Time `json:"finished_at,omitempty"`
}

type RawSnapshot struct {
	ID            string            `json:"id"`
	JobID         string            `json:"job_id"`
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
	ID                   string            `json:"id"`
	JobID                string            `json:"job_id"`
	Provider             string            `json:"provider"`
	Pipeline             Pipeline          `json:"pipeline"`
	ParserVersion        string            `json:"parser_version"`
	NormalizationVersion string            `json:"normalization_version"`
	CreatedAt            time.Time         `json:"created_at"`
	Metadata             map[string]string `json:"metadata,omitempty"`
	Payload              contract.Envelope `json:"payload"`
}

type PublishedResult struct {
	ID              string            `json:"id"`
	JobID           string            `json:"job_id"`
	Provider        string            `json:"provider"`
	Pipeline        Pipeline          `json:"pipeline"`
	ContractVersion string            `json:"contract_version"`
	Checksum        string            `json:"checksum"`
	PublishedAt     time.Time         `json:"published_at"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	Payload         contract.Envelope `json:"payload"`
}

type Schedule struct {
	Name           string    `json:"name"`
	CronExpression string    `json:"cron_expression"`
	Enabled        bool      `json:"enabled"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func NewID(prefix string) string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return prefix + "_" + time.Now().UTC().Format("20060102T150405.000000000") + "_" + hex.EncodeToString(buf)
}

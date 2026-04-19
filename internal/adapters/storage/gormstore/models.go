package gormstore

import "time"

type jobModel struct {
	ID                   string `gorm:"primaryKey"`
	Provider             string `gorm:"index;not null"`
	Pipeline             string `gorm:"index;not null"`
	Trigger              string `gorm:"not null"`
	State                string `gorm:"index;not null"`
	CorrelationID        string `gorm:"index"`
	ParserVersion        string `gorm:"not null"`
	NormalizationVersion string `gorm:"not null"`
	Country              string
	EventType            string
	EventID              string
	EventURL             string
	EventName            string
	RequestJSON          []byte `gorm:"type:blob"`
	StatsJSON            []byte `gorm:"type:blob"`
	ErrorJSON            []byte `gorm:"type:blob"`
	ErrorCategory        string
	ErrorCode            string
	ErrorMessage         string
	ErrorRetryable       bool
	AttemptCount         int    `gorm:"not null"`
	MaxAttempts          int    `gorm:"not null"`
	ClaimedBy            string `gorm:"index"`
	ClaimedAt            *time.Time
	LeaseUntil           *time.Time `gorm:"index"`
	LastHeartbeatAt      *time.Time
	NextRetryAt          *time.Time `gorm:"index"`
	LastTransitionAt     time.Time  `gorm:"index;not null"`
	CreatedAt            time.Time  `gorm:"index;not null"`
	UpdatedAt            time.Time  `gorm:"index;not null"`
	StartedAt            *time.Time
	FinishedAt           *time.Time
}

func (jobModel) TableName() string { return "ingestion_jobs" }

type jobAttemptModel struct {
	ID              string `gorm:"primaryKey"`
	JobID           string `gorm:"index;not null"`
	AttemptNumber   int    `gorm:"not null"`
	WorkerID        string `gorm:"index;not null"`
	State           string `gorm:"not null"`
	ErrorJSON       []byte `gorm:"type:blob"`
	ErrorCategory   string
	ErrorCode       string
	ErrorMessage    string
	ErrorRetryable  bool
	StartedAt       time.Time `gorm:"index;not null"`
	FinishedAt      *time.Time
	LastHeartbeatAt *time.Time
	LeaseUntil      *time.Time `gorm:"index"`
	CreatedAt       time.Time  `gorm:"index;not null"`
}

func (jobAttemptModel) TableName() string { return "job_attempts" }

type jobTransitionModel struct {
	ID            string `gorm:"primaryKey"`
	JobID         string `gorm:"index;not null"`
	AttemptNumber *int
	FromState     string
	ToState       string `gorm:"index;not null"`
	Reason        string
	MetadataJSON  []byte    `gorm:"type:blob"`
	CreatedAt     time.Time `gorm:"index;not null"`
}

func (jobTransitionModel) TableName() string { return "job_state_transitions" }

type rawSnapshotModel struct {
	ID            string `gorm:"primaryKey"`
	JobID         string `gorm:"index;not null"`
	AttemptNumber int    `gorm:"not null"`
	Provider      string `gorm:"index;not null"`
	Pipeline      string `gorm:"index;not null"`
	ResourceType  string `gorm:"index;not null"`
	ResourceKey   string `gorm:"index;not null"`
	SourceURL     string
	ContentType   string
	StatusCode    int
	ParserVersion string
	CapturedAt    time.Time `gorm:"index;not null"`
	SHA256        string    `gorm:"index;not null"`
	Body          []byte    `gorm:"type:blob"`
	MetadataJSON  []byte    `gorm:"type:blob"`
}

func (rawSnapshotModel) TableName() string { return "raw_snapshots" }

type normalizedResultModel struct {
	ID                   string `gorm:"primaryKey"`
	JobID                string `gorm:"index;not null"`
	AttemptNumber        int    `gorm:"not null"`
	Provider             string `gorm:"index;not null"`
	Pipeline             string `gorm:"index;not null"`
	ScopeKey             string `gorm:"index"`
	ParserVersion        string `gorm:"not null"`
	NormalizationVersion string `gorm:"not null"`
	ContractVersion      string
	SourceSnapshotHash   string
	PayloadHash          string `gorm:"index;not null"`
	PublicationDecision  string
	PublicationReason    string
	ChangeClassification string
	ForcedRepublish      bool      `gorm:"not null;default:false"`
	CreatedAt            time.Time `gorm:"index;not null"`
	MetadataJSON         []byte    `gorm:"type:blob"`
	PayloadJSON          []byte    `gorm:"type:blob"`
}

func (normalizedResultModel) TableName() string { return "normalized_results" }

type publishedResultModel struct {
	ID                      string `gorm:"primaryKey"`
	JobID                   string `gorm:"index;not null"`
	AttemptNumber           int    `gorm:"not null"`
	Provider                string `gorm:"index;not null"`
	Pipeline                string `gorm:"index;not null"`
	ScopeKey                string `gorm:"index"`
	CorrelationID           string `gorm:"index"`
	ParserVersion           string
	NormalizationVersion    string
	ContractVersion         string `gorm:"not null"`
	SourceSnapshotHash      string
	NormalizedHash          string
	Checksum                string `gorm:"index;not null"`
	PublicationDecision     string
	PublicationReason       string
	ChangeClassification    string
	ForcedRepublish         bool `gorm:"not null;default:false"`
	SupersedesPublicationID string
	PublishedAt             time.Time `gorm:"index;not null"`
	MetadataJSON            []byte    `gorm:"type:blob"`
	PayloadJSON             []byte    `gorm:"type:blob"`
}

func (publishedResultModel) TableName() string { return "published_results" }

type scheduleModel struct {
	Name           string    `gorm:"primaryKey"`
	CronExpression string    `gorm:"not null"`
	Enabled        bool      `gorm:"not null"`
	UpdatedAt      time.Time `gorm:"index;not null"`
}

func (scheduleModel) TableName() string { return "schedule_configs_v2" }

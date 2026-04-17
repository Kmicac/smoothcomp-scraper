package audit

import (
	"time"

	"github.com/kmicac/smoothcomp-scraper/internal/core/job"
)

const (
	MatchExact            = "exact"
	MatchPartial          = "partial"
	MatchSourceNotVisible = "source_not_visible"

	CategorySourceNotVisible  = "SOURCE_NOT_VISIBLE"
	CategoryPartialSource     = "PARTIAL_SOURCE_DATA"
	CategoryParserDrift       = "PARSER_DRIFT"
	CategoryNormalizationBug  = "NORMALIZATION_BUG"
	CategoryIDResolutionBug   = "ID_RESOLUTION_BUG"
	CategorySubdomainVariant  = "SUBDOMAIN_VARIANT"
	CategoryExpectationWrong  = "EXPECTATION_WAS_WRONG"
	CategoryUnsupported       = "UNSUPPORTED_VARIANT"
	CategoryUnsupportedLayout = "UNSUPPORTED_LAYOUT"
)

type Dataset struct {
	Name  string `json:"name"`
	Cases []Case `json:"cases"`
}

type Case struct {
	ID           string       `json:"id"`
	Pipeline     job.Pipeline `json:"pipeline"`
	Description  string       `json:"description"`
	Tags         []string     `json:"tags,omitempty"`
	Request      job.Request  `json:"request"`
	Snapshots    []Snapshot   `json:"snapshots"`
	Expectations Expectations `json:"expectations"`
}

type Snapshot struct {
	ID           string            `json:"id"`
	ResourceType string            `json:"resource_type"`
	ResourceKey  string            `json:"resource_key,omitempty"`
	SourceURL    string            `json:"source_url,omitempty"`
	ContentType  string            `json:"content_type,omitempty"`
	StatusCode   int               `json:"status_code"`
	File         string            `json:"file"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type Expectations struct {
	Scope          map[string]FieldExpectation            `json:"scope,omitempty"`
	Events         map[string]map[string]FieldExpectation `json:"events,omitempty"`
	Organizations  map[string]map[string]FieldExpectation `json:"organizations,omitempty"`
	People         map[string]map[string]FieldExpectation `json:"people,omitempty"`
	Registrations  map[string]map[string]FieldExpectation `json:"registrations,omitempty"`
	Matches        map[string]map[string]FieldExpectation `json:"matches,omitempty"`
	MatchSummaries map[string]map[string]FieldExpectation `json:"match_summaries,omitempty"`
	Warnings       WarningExpectations                    `json:"warnings,omitempty"`
}

type FieldExpectation struct {
	Match            string `json:"match"`
	Value            string `json:"value,omitempty"`
	MismatchCategory string `json:"mismatch_category,omitempty"`
	Notes            string `json:"notes,omitempty"`
}

type WarningExpectations struct {
	Required  []WarningExpectation `json:"required,omitempty"`
	Forbidden []WarningExpectation `json:"forbidden,omitempty"`
}

type WarningExpectation struct {
	Code             string `json:"code"`
	SubjectType      string `json:"subject_type,omitempty"`
	SubjectID        string `json:"subject_id,omitempty"`
	MismatchCategory string `json:"mismatch_category,omitempty"`
	Notes            string `json:"notes,omitempty"`
}

type Report struct {
	DatasetName        string               `json:"dataset_name"`
	GeneratedAt        time.Time            `json:"generated_at"`
	CaseCount          int                  `json:"case_count"`
	CaseResults        []CaseResult         `json:"case_results"`
	FieldReliability   []FieldReliability   `json:"field_reliability"`
	WarningReliability []WarningReliability `json:"warning_reliability"`
	Summary            Summary              `json:"summary"`
}

type Summary struct {
	ExactMatches     int `json:"exact_matches"`
	PartialMatches   int `json:"partial_matches"`
	Mismatches       int `json:"mismatches"`
	UnsupportedFacts int `json:"unsupported_facts"`
	ExpectedWarnings int `json:"expected_warnings"`
	MissingWarnings  int `json:"missing_warnings"`
	UnexpectedWarns  int `json:"unexpected_warnings"`
}

type CaseResult struct {
	ID               string           `json:"id"`
	Pipeline         job.Pipeline     `json:"pipeline"`
	Description      string           `json:"description"`
	Tags             []string         `json:"tags,omitempty"`
	ExactMatches     int              `json:"exact_matches"`
	PartialMatches   int              `json:"partial_matches"`
	UnsupportedFacts int              `json:"unsupported_facts"`
	Mismatches       []Mismatch       `json:"mismatches,omitempty"`
	MissingWarnings  []WarningFinding `json:"missing_warnings,omitempty"`
	UnexpectedWarns  []WarningFinding `json:"unexpected_warnings,omitempty"`
	ActualWarnings   []WarningFinding `json:"actual_warnings,omitempty"`
}

type Mismatch struct {
	Pipeline       job.Pipeline `json:"pipeline"`
	CaseID         string       `json:"case_id"`
	EntityType     string       `json:"entity_type"`
	EntityID       string       `json:"entity_id,omitempty"`
	Field          string       `json:"field"`
	Expected       string       `json:"expected,omitempty"`
	Actual         string       `json:"actual,omitempty"`
	Classification string       `json:"classification"`
	Notes          string       `json:"notes,omitempty"`
}

type WarningFinding struct {
	Pipeline       job.Pipeline `json:"pipeline"`
	CaseID         string       `json:"case_id"`
	Code           string       `json:"code"`
	SubjectType    string       `json:"subject_type,omitempty"`
	SubjectID      string       `json:"subject_id,omitempty"`
	Classification string       `json:"classification,omitempty"`
	Notes          string       `json:"notes,omitempty"`
}

type FieldReliability struct {
	Pipeline         job.Pipeline `json:"pipeline"`
	Field            string       `json:"field"`
	VisibleCases     int          `json:"visible_cases"`
	ProducedCases    int          `json:"produced_cases"`
	ExactMatches     int          `json:"exact_matches"`
	PartialMatches   int          `json:"partial_matches"`
	Mismatches       int          `json:"mismatches"`
	UnsupportedCases int          `json:"unsupported_cases"`
	Coverage         float64      `json:"coverage"`
	ExactRate        float64      `json:"exact_rate"`
	PartialRate      float64      `json:"partial_rate"`
	Confidence       string       `json:"confidence"`
	RiskNotes        []string     `json:"risk_notes,omitempty"`
}

type WarningReliability struct {
	Pipeline   job.Pipeline `json:"pipeline"`
	Code       string       `json:"code"`
	Expected   int          `json:"expected"`
	Matched    int          `json:"matched"`
	Missing    int          `json:"missing"`
	Unexpected int          `json:"unexpected"`
	Confidence string       `json:"confidence"`
	RiskNotes  []string     `json:"risk_notes,omitempty"`
}

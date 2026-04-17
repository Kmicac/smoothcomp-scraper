package contract

import "time"

const CurrentContractVersion = "smoothcomp.adapter.contract.v1"

type Envelope struct {
	ContractVersion      string            `json:"contract_version"`
	Provider             string            `json:"provider"`
	Pipeline             string            `json:"pipeline"`
	JobID                string            `json:"job_id"`
	CorrelationID        string            `json:"correlation_id"`
	ParserVersion        string            `json:"parser_version"`
	NormalizationVersion string            `json:"normalization_version"`
	GeneratedAt          time.Time         `json:"generated_at"`
	Scope                Scope             `json:"scope"`
	Events               []Event           `json:"events,omitempty"`
	Organizations        []Organization    `json:"organizations,omitempty"`
	People               []Person          `json:"people,omitempty"`
	Registrations        []Registration    `json:"registrations,omitempty"`
	Matches              []Match           `json:"matches,omitempty"`
	MatchSummaries       []MatchSummary    `json:"match_summaries,omitempty"`
	Warnings             []Warning         `json:"warnings,omitempty"`
	Metadata             map[string]string `json:"metadata,omitempty"`
}

type Scope struct {
	Country   string `json:"country,omitempty"`
	EventType string `json:"event_type,omitempty"`
	EventID   string `json:"event_id,omitempty"`
	ProfileID string `json:"profile_id,omitempty"`
}

type Event struct {
	SourceID        string            `json:"source_id"`
	SourceURL       string            `json:"source_url,omitempty"`
	Name            string            `json:"name,omitempty"`
	Description     string            `json:"description,omitempty"`
	ImageURL        string            `json:"image_url,omitempty"`
	City            string            `json:"city,omitempty"`
	Country         string            `json:"country,omitempty"`
	CountryCode     string            `json:"country_code,omitempty"`
	VenueName       string            `json:"venue_name,omitempty"`
	VenueAddress    string            `json:"venue_address,omitempty"`
	OrganizerName   string            `json:"organizer_name,omitempty"`
	Status          string            `json:"status,omitempty"`
	StartsAt        string            `json:"starts_at,omitempty"`
	EndsAt          string            `json:"ends_at,omitempty"`
	RawReferenceIDs []string          `json:"raw_reference_ids,omitempty"`
	Attributes      map[string]string `json:"attributes,omitempty"`
}

type Organization struct {
	SourceID        string            `json:"source_id"`
	Name            string            `json:"name"`
	Kind            string            `json:"kind"`
	Country         string            `json:"country,omitempty"`
	CountryCode     string            `json:"country_code,omitempty"`
	Slug            string            `json:"slug,omitempty"`
	ImageURL        string            `json:"image_url,omitempty"`
	Description     string            `json:"description,omitempty"`
	WebsiteURL      string            `json:"website_url,omitempty"`
	RawReferenceIDs []string          `json:"raw_reference_ids,omitempty"`
	Attributes      map[string]string `json:"attributes,omitempty"`
}

type Person struct {
	SourceID             string            `json:"source_id"`
	GivenName            string            `json:"given_name,omitempty"`
	FamilyName           string            `json:"family_name,omitempty"`
	FullName             string            `json:"full_name,omitempty"`
	Country              string            `json:"country,omitempty"`
	CountryCode          string            `json:"country_code,omitempty"`
	Gender               string            `json:"gender,omitempty"`
	Age                  *int              `json:"age,omitempty"`
	BeltRank             string            `json:"belt_rank,omitempty"`
	BirthYear            *int              `json:"birth_year,omitempty"`
	ProfileURL           string            `json:"profile_url,omitempty"`
	ImageURL             string            `json:"image_url,omitempty"`
	OrganizationSourceID string            `json:"organization_source_id,omitempty"`
	RawReferenceIDs      []string          `json:"raw_reference_ids,omitempty"`
	Attributes           map[string]string `json:"attributes,omitempty"`
}

type Registration struct {
	SourceID             string            `json:"source_id"`
	EventSourceID        string            `json:"event_source_id"`
	PersonSourceID       string            `json:"person_source_id"`
	OrganizationSourceID string            `json:"organization_source_id,omitempty"`
	Division             string            `json:"division,omitempty"`
	AgeCategory          string            `json:"age_category,omitempty"`
	Rank                 string            `json:"rank,omitempty"`
	WeightClass          string            `json:"weight_class,omitempty"`
	ActualWeight         *float64          `json:"actual_weight,omitempty"`
	Seed                 *int              `json:"seed,omitempty"`
	RawReferenceIDs      []string          `json:"raw_reference_ids,omitempty"`
	Attributes           map[string]string `json:"attributes,omitempty"`
}

type Match struct {
	SourceID         string            `json:"source_id"`
	SourceURL        string            `json:"source_url,omitempty"`
	EventSourceID    string            `json:"event_source_id,omitempty"`
	EventName        string            `json:"event_name,omitempty"`
	AthleteSourceID  string            `json:"athlete_source_id,omitempty"`
	OpponentSourceID string            `json:"opponent_source_id,omitempty"`
	OpponentName     string            `json:"opponent_name,omitempty"`
	OpponentCountry  string            `json:"opponent_country,omitempty"`
	Division         string            `json:"division,omitempty"`
	AgeCategory      string            `json:"age_category,omitempty"`
	Rank             string            `json:"rank,omitempty"`
	WeightClass      string            `json:"weight_class,omitempty"`
	Outcome          string            `json:"outcome,omitempty"`
	FinishMethod     string            `json:"finish_method,omitempty"`
	ResultText       string            `json:"result_text,omitempty"`
	ScoreText        string            `json:"score_text,omitempty"`
	BracketLabel     string            `json:"bracket_label,omitempty"`
	RoundLabel       string            `json:"round_label,omitempty"`
	Placement        string            `json:"placement,omitempty"`
	Confidence       string            `json:"confidence,omitempty"`
	StartsAt         string            `json:"starts_at,omitempty"`
	RawReferenceIDs  []string          `json:"raw_reference_ids,omitempty"`
	Attributes       map[string]string `json:"attributes,omitempty"`
}

type MatchSummary struct {
	AthleteSourceID    string            `json:"athlete_source_id"`
	Scope              string            `json:"scope,omitempty"`
	Confidence         string            `json:"confidence,omitempty"`
	TotalMatches       int               `json:"total_matches"`
	TotalWins          int               `json:"total_wins"`
	TotalLosses        int               `json:"total_losses"`
	WinsBySubmission   int               `json:"wins_by_submission"`
	WinsByPoints       int               `json:"wins_by_points"`
	WinsByDecision     int               `json:"wins_by_decision"`
	WinsByDQ           int               `json:"wins_by_dq"`
	LossesBySubmission int               `json:"losses_by_submission"`
	LossesByPoints     int               `json:"losses_by_points"`
	LossesByDecision   int               `json:"losses_by_decision"`
	LossesByDQ         int               `json:"losses_by_dq"`
	RawReferenceIDs    []string          `json:"raw_reference_ids,omitempty"`
	Attributes         map[string]string `json:"attributes,omitempty"`
}

type Warning struct {
	Code             string `json:"code"`
	Message          string `json:"message"`
	SubjectType      string `json:"subject_type,omitempty"`
	SubjectID        string `json:"subject_id,omitempty"`
	SourceSnapshotID string `json:"source_snapshot_id,omitempty"`
}

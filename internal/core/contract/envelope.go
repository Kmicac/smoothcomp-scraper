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
	Metadata             map[string]string `json:"metadata,omitempty"`
}

type Scope struct {
	Country   string `json:"country,omitempty"`
	EventType string `json:"event_type,omitempty"`
	EventID   string `json:"event_id,omitempty"`
}

type Event struct {
	SourceID        string            `json:"source_id"`
	SourceURL       string            `json:"source_url,omitempty"`
	Name            string            `json:"name,omitempty"`
	City            string            `json:"city,omitempty"`
	Country         string            `json:"country,omitempty"`
	CountryCode     string            `json:"country_code,omitempty"`
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
	CountryCode     string            `json:"country_code,omitempty"`
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

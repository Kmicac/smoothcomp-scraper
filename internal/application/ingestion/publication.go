package ingestion

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/kmicac/smoothcomp-scraper/internal/core/contract"
	"github.com/kmicac/smoothcomp-scraper/internal/core/job"
	corepublication "github.com/kmicac/smoothcomp-scraper/internal/core/publication"
)

const (
	publicationReasonFirstPublication            = "first_effective_publication"
	publicationReasonSourceChanged               = "source_snapshot_hash_changed"
	publicationReasonParserVersionChanged        = "parser_version_changed"
	publicationReasonNormalizationVersionChanged = "normalization_version_changed"
	publicationReasonNormalizedHashChanged       = "normalized_hash_changed"
	publicationReasonContractVersionChanged      = "contract_version_changed"
	publicationReasonEnvelopeHashChanged         = "envelope_hash_changed"
	publicationReasonNoChange                    = "no_change_detected"
	publicationReasonForcedRepublish             = "forced_republish_requested"

	requestMetadataForceRepublish  = "force_republish"
	requestMetadataImportRequestID = "import_request_id"

	envelopeMetadataScopeKey                = "scope_key"
	envelopeMetadataSourceSnapshotHash      = "source_snapshot_hash"
	envelopeMetadataNormalizedHash          = "normalized_hash"
	envelopeMetadataEnvelopeHash            = "envelope_hash"
	envelopeMetadataPublishedAt             = "published_at"
	envelopeMetadataPublicationDecision     = "publication_decision"
	envelopeMetadataPublicationReason       = "publication_reason"
	envelopeMetadataChangeClassification    = "change_classification"
	envelopeMetadataForcedRepublish         = "forced_republish"
	envelopeMetadataSupersedesPublicationID = "supersedes_publication_id"
	envelopeMetadataImportRequestID         = "import_request_id"
	envelopeMetadataSourceProfileID         = "source_profile_id"
	envelopeMetadataSourceEventID           = "source_event_id"
)

type publicationCandidate struct {
	ScopeKey             string
	SourceSnapshotHash   string
	NormalizedHash       string
	EnvelopeHash         string
	ParserVersion        string
	NormalizationVersion string
	ContractVersion      string
	ForcedRepublish      bool
}

type publicationOutcome struct {
	Decision                job.PublicationDecision
	Reason                  string
	Classification          job.ChangeClassification
	SupersedesPublicationID string
}

func computeScopeKey(provider string, pipeline job.Pipeline, scope contract.Scope) string {
	return corepublication.ScopeKey(provider, pipeline, scope)
}

func computeSourceSnapshotHash(snapshots []job.RawSnapshot) (string, error) {
	type sourceSnapshotFingerprint struct {
		ResourceType string `json:"resource_type"`
		ResourceKey  string `json:"resource_key"`
		SourceURL    string `json:"source_url,omitempty"`
		ContentType  string `json:"content_type,omitempty"`
		StatusCode   int    `json:"status_code"`
		SHA256       string `json:"sha256"`
	}

	items := make([]sourceSnapshotFingerprint, 0, len(snapshots))
	for _, snapshot := range snapshots {
		items = append(items, sourceSnapshotFingerprint{
			ResourceType: snapshot.ResourceType,
			ResourceKey:  snapshot.ResourceKey,
			SourceURL:    snapshot.SourceURL,
			ContentType:  snapshot.ContentType,
			StatusCode:   snapshot.StatusCode,
			SHA256:       snapshot.SHA256,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if left.ResourceType != right.ResourceType {
			return left.ResourceType < right.ResourceType
		}
		if left.ResourceKey != right.ResourceKey {
			return left.ResourceKey < right.ResourceKey
		}
		if left.SourceURL != right.SourceURL {
			return left.SourceURL < right.SourceURL
		}
		if left.StatusCode != right.StatusCode {
			return left.StatusCode < right.StatusCode
		}
		return left.SHA256 < right.SHA256
	})
	return hashJSON(items)
}

func computeNormalizedHash(payload contract.Envelope) (string, error) {
	canonical := canonicalizeEnvelope(payload)
	canonical.Metadata = nil
	return hashJSON(canonical)
}

func computeEnvelopeHash(payload contract.Envelope) (string, error) {
	canonical := canonicalizeEnvelope(payload)
	if len(canonical.Metadata) > 0 {
		filtered := map[string]string{}
		for key, value := range canonical.Metadata {
			switch key {
			case envelopeMetadataEnvelopeHash,
				envelopeMetadataPublishedAt,
				envelopeMetadataPublicationDecision,
				envelopeMetadataPublicationReason,
				envelopeMetadataChangeClassification,
				envelopeMetadataForcedRepublish,
				envelopeMetadataSupersedesPublicationID,
				envelopeMetadataImportRequestID:
				continue
			default:
				filtered[key] = value
			}
		}
		if len(filtered) == 0 {
			canonical.Metadata = nil
		} else {
			canonical.Metadata = filtered
		}
	}
	return hashJSON(canonical)
}

func canonicalizeEnvelope(payload contract.Envelope) contract.Envelope {
	canonical := payload
	canonical.JobID = ""
	canonical.CorrelationID = ""
	canonical.GeneratedAt = time.Time{}
	canonical.Metadata = copyStringMap(payload.Metadata)

	canonical.Events = make([]contract.Event, len(payload.Events))
	for i, item := range payload.Events {
		canonical.Events[i] = item
		canonical.Events[i].RawReferenceIDs = nil
		canonical.Events[i].Attributes = copyStringMap(item.Attributes)
	}

	canonical.Organizations = make([]contract.Organization, len(payload.Organizations))
	for i, item := range payload.Organizations {
		canonical.Organizations[i] = item
		canonical.Organizations[i].RawReferenceIDs = nil
		canonical.Organizations[i].Attributes = copyStringMap(item.Attributes)
	}

	canonical.People = make([]contract.Person, len(payload.People))
	for i, item := range payload.People {
		canonical.People[i] = item
		canonical.People[i].RawReferenceIDs = nil
		canonical.People[i].Attributes = copyStringMap(item.Attributes)
	}

	canonical.Registrations = make([]contract.Registration, len(payload.Registrations))
	for i, item := range payload.Registrations {
		canonical.Registrations[i] = item
		canonical.Registrations[i].RawReferenceIDs = nil
		canonical.Registrations[i].Attributes = copyStringMap(item.Attributes)
	}

	canonical.Matches = make([]contract.Match, len(payload.Matches))
	for i, item := range payload.Matches {
		canonical.Matches[i] = item
		canonical.Matches[i].RawReferenceIDs = nil
		canonical.Matches[i].Attributes = copyStringMap(item.Attributes)
	}

	canonical.MatchSummaries = make([]contract.MatchSummary, len(payload.MatchSummaries))
	for i, item := range payload.MatchSummaries {
		canonical.MatchSummaries[i] = item
		canonical.MatchSummaries[i].RawReferenceIDs = nil
		canonical.MatchSummaries[i].Attributes = copyStringMap(item.Attributes)
	}

	canonical.Warnings = make([]contract.Warning, len(payload.Warnings))
	for i, item := range payload.Warnings {
		canonical.Warnings[i] = item
		canonical.Warnings[i].SourceSnapshotID = ""
	}

	return canonical
}

func decidePublication(previous *job.PublishedResult, candidate publicationCandidate) publicationOutcome {
	if candidate.ForcedRepublish {
		outcome := publicationOutcome{
			Decision:       job.PublicationDecisionPublishForced,
			Reason:         publicationReasonForcedRepublish,
			Classification: job.ChangeClassificationRepublishForced,
		}
		if previous != nil {
			outcome.SupersedesPublicationID = previous.ID
		}
		return outcome
	}
	if previous == nil {
		return publicationOutcome{
			Decision:       job.PublicationDecisionPublishChanged,
			Reason:         publicationReasonFirstPublication,
			Classification: job.ChangeClassificationContentChanged,
		}
	}
	if previous.SourceSnapshotHash != candidate.SourceSnapshotHash {
		return publicationOutcome{
			Decision:                job.PublicationDecisionPublishChanged,
			Reason:                  publicationReasonSourceChanged,
			Classification:          job.ChangeClassificationContentChanged,
			SupersedesPublicationID: previous.ID,
		}
	}
	if previous.ParserVersion != candidate.ParserVersion {
		return publicationOutcome{
			Decision:                job.PublicationDecisionPublishChanged,
			Reason:                  publicationReasonParserVersionChanged,
			Classification:          job.ChangeClassificationNormalizationChanged,
			SupersedesPublicationID: previous.ID,
		}
	}
	if previous.NormalizationVersion != candidate.NormalizationVersion {
		return publicationOutcome{
			Decision:                job.PublicationDecisionPublishChanged,
			Reason:                  publicationReasonNormalizationVersionChanged,
			Classification:          job.ChangeClassificationNormalizationChanged,
			SupersedesPublicationID: previous.ID,
		}
	}
	if previous.ContractVersion != candidate.ContractVersion {
		return publicationOutcome{
			Decision:                job.PublicationDecisionPublishChanged,
			Reason:                  publicationReasonContractVersionChanged,
			Classification:          job.ChangeClassificationNormalizationChanged,
			SupersedesPublicationID: previous.ID,
		}
	}
	if previous.NormalizedHash != candidate.NormalizedHash {
		return publicationOutcome{
			Decision:                job.PublicationDecisionPublishChanged,
			Reason:                  publicationReasonNormalizedHashChanged,
			Classification:          job.ChangeClassificationNormalizationChanged,
			SupersedesPublicationID: previous.ID,
		}
	}
	if previous.EnvelopeHash != candidate.EnvelopeHash {
		return publicationOutcome{
			Decision:                job.PublicationDecisionPublishChanged,
			Reason:                  publicationReasonEnvelopeHashChanged,
			Classification:          job.ChangeClassificationNormalizationChanged,
			SupersedesPublicationID: previous.ID,
		}
	}
	return publicationOutcome{
		Decision:       job.PublicationDecisionSkipNoChange,
		Reason:         publicationReasonNoChange,
		Classification: job.ChangeClassificationNoChange,
	}
}

func forceRepublish(request job.Request) bool {
	if request.Metadata == nil {
		return false
	}
	value := strings.ToLower(strings.TrimSpace(request.Metadata[requestMetadataForceRepublish]))
	return value == "1" || value == "true" || value == "yes" || value == "y"
}

func buildPublishedEnvelopeMetadata(
	request job.Request,
	scope contract.Scope,
	scopeKey string,
	sourceSnapshotHash string,
	normalizedHash string,
	envelopeHash string,
	outcome publicationOutcome,
	publishedAt time.Time,
) map[string]string {
	metadata := map[string]string{
		envelopeMetadataScopeKey:             scopeKey,
		envelopeMetadataSourceSnapshotHash:   sourceSnapshotHash,
		envelopeMetadataNormalizedHash:       normalizedHash,
		envelopeMetadataEnvelopeHash:         envelopeHash,
		envelopeMetadataPublishedAt:          publishedAt.UTC().Format(time.RFC3339Nano),
		envelopeMetadataPublicationDecision:  string(outcome.Decision),
		envelopeMetadataPublicationReason:    outcome.Reason,
		envelopeMetadataChangeClassification: string(outcome.Classification),
		envelopeMetadataForcedRepublish:      boolString(outcome.Decision == job.PublicationDecisionPublishForced),
	}
	if outcome.SupersedesPublicationID != "" {
		metadata[envelopeMetadataSupersedesPublicationID] = outcome.SupersedesPublicationID
	}
	if request.Metadata != nil && strings.TrimSpace(request.Metadata[requestMetadataImportRequestID]) != "" {
		metadata[envelopeMetadataImportRequestID] = strings.TrimSpace(request.Metadata[requestMetadataImportRequestID])
	}
	if scope.ProfileID != "" {
		metadata[envelopeMetadataSourceProfileID] = scope.ProfileID
	}
	if scope.EventID != "" {
		metadata[envelopeMetadataSourceEventID] = scope.EventID
	}
	return metadata
}

func mergeEnvelopeMetadata(base map[string]string, extra map[string]string) map[string]string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	merged := copyStringMap(base)
	if merged == nil {
		merged = map[string]string{}
	}
	for key, value := range extra {
		if strings.TrimSpace(value) == "" {
			continue
		}
		merged[key] = value
	}
	return merged
}

func hashJSON(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func copyStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

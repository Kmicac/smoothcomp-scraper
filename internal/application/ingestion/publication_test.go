package ingestion

import (
	"testing"
	"time"

	"github.com/kmicac/smoothcomp-scraper/internal/core/contract"
	"github.com/kmicac/smoothcomp-scraper/internal/core/job"
)

func TestPublicationHashesIgnoreVolatileExecutionFields(t *testing.T) {
	first := contract.Envelope{
		ContractVersion:      contract.CurrentContractVersion,
		Provider:             "smoothcomp",
		Pipeline:             string(job.PipelineSmoothcompAthleteProfile),
		JobID:                "job_1",
		CorrelationID:        "corr_1",
		ParserVersion:        "parser.v1",
		NormalizationVersion: "norm.v1",
		GeneratedAt:          time.Now().UTC(),
		Scope:                contract.Scope{ProfileID: "profile_123"},
		People: []contract.Person{{
			SourceID:        "profile_123",
			FullName:        "Jane Doe",
			RawReferenceIDs: []string{"snap_1"},
			Attributes:      map[string]string{"belt_rank": "brown"},
		}},
		Warnings: []contract.Warning{{
			Code:             "athlete_profile_events_unavailable",
			Message:          "events unavailable",
			SourceSnapshotID: "snap_1",
		}},
		Metadata: map[string]string{
			"primary_snapshot_id": "snap_1",
		},
	}

	second := first
	second.JobID = "job_2"
	second.CorrelationID = "corr_2"
	second.GeneratedAt = first.GeneratedAt.Add(5 * time.Minute)
	second.People = []contract.Person{{
		SourceID:        "profile_123",
		FullName:        "Jane Doe",
		RawReferenceIDs: []string{"snap_2"},
		Attributes:      map[string]string{"belt_rank": "brown"},
	}}
	second.Warnings = []contract.Warning{{
		Code:             "athlete_profile_events_unavailable",
		Message:          "events unavailable",
		SourceSnapshotID: "snap_2",
	}}
	second.Metadata = map[string]string{
		"primary_snapshot_id": "snap_2",
	}

	firstNormalizedHash, err := computeNormalizedHash(first)
	if err != nil {
		t.Fatalf("compute first normalized hash: %v", err)
	}
	secondNormalizedHash, err := computeNormalizedHash(second)
	if err != nil {
		t.Fatalf("compute second normalized hash: %v", err)
	}
	if firstNormalizedHash != secondNormalizedHash {
		t.Fatalf("expected normalized hashes to match, got %s and %s", firstNormalizedHash, secondNormalizedHash)
	}
}

func TestDecidePublicationClassifiesContentAndNormalizationChanges(t *testing.T) {
	base := &job.PublishedResult{
		ID:                   "pub_prev",
		SourceSnapshotHash:   "source_hash_1",
		NormalizedHash:       "normalized_hash_1",
		EnvelopeHash:         "envelope_hash_1",
		ParserVersion:        "parser.v1",
		NormalizationVersion: "norm.v1",
		ContractVersion:      contract.CurrentContractVersion,
	}

	contentChanged := decidePublication(base, publicationCandidate{
		SourceSnapshotHash:   "source_hash_2",
		NormalizedHash:       "normalized_hash_1",
		EnvelopeHash:         "envelope_hash_2",
		ParserVersion:        "parser.v1",
		NormalizationVersion: "norm.v1",
		ContractVersion:      contract.CurrentContractVersion,
	})
	if contentChanged.Decision != job.PublicationDecisionPublishChanged {
		t.Fatalf("expected content change to publish, got %s", contentChanged.Decision)
	}
	if contentChanged.Classification != job.ChangeClassificationContentChanged {
		t.Fatalf("expected CONTENT_CHANGED, got %s", contentChanged.Classification)
	}

	normalizationChanged := decidePublication(base, publicationCandidate{
		SourceSnapshotHash:   "source_hash_1",
		NormalizedHash:       "normalized_hash_2",
		EnvelopeHash:         "envelope_hash_2",
		ParserVersion:        "parser.v2",
		NormalizationVersion: "norm.v1",
		ContractVersion:      contract.CurrentContractVersion,
	})
	if normalizationChanged.Decision != job.PublicationDecisionPublishChanged {
		t.Fatalf("expected normalization change to publish, got %s", normalizationChanged.Decision)
	}
	if normalizationChanged.Classification != job.ChangeClassificationNormalizationChanged {
		t.Fatalf("expected NORMALIZATION_CHANGED, got %s", normalizationChanged.Classification)
	}

	noChange := decidePublication(base, publicationCandidate{
		SourceSnapshotHash:   "source_hash_1",
		NormalizedHash:       "normalized_hash_1",
		EnvelopeHash:         "envelope_hash_1",
		ParserVersion:        "parser.v1",
		NormalizationVersion: "norm.v1",
		ContractVersion:      contract.CurrentContractVersion,
	})
	if noChange.Decision != job.PublicationDecisionSkipNoChange {
		t.Fatalf("expected no change to skip, got %s", noChange.Decision)
	}
	if noChange.Classification != job.ChangeClassificationNoChange {
		t.Fatalf("expected NO_CHANGE, got %s", noChange.Classification)
	}

	forced := decidePublication(base, publicationCandidate{
		SourceSnapshotHash:   "source_hash_1",
		NormalizedHash:       "normalized_hash_1",
		EnvelopeHash:         "envelope_hash_1",
		ParserVersion:        "parser.v1",
		NormalizationVersion: "norm.v1",
		ContractVersion:      contract.CurrentContractVersion,
		ForcedRepublish:      true,
	})
	if forced.Decision != job.PublicationDecisionPublishForced {
		t.Fatalf("expected forced republish to publish, got %s", forced.Decision)
	}
	if forced.Classification != job.ChangeClassificationRepublishForced {
		t.Fatalf("expected REPUBLISH_FORCED, got %s", forced.Classification)
	}
}

func TestComputeSourceSnapshotHashIgnoresSnapshotIDs(t *testing.T) {
	first := []job.RawSnapshot{{
		ID:           "snap_1",
		ResourceType: "athlete_profile_html",
		ResourceKey:  "profile_123",
		SourceURL:    "https://smoothcomp.com/en/profile/profile_123",
		ContentType:  "text/html",
		StatusCode:   200,
		SHA256:       "body_hash_1",
	}}
	second := []job.RawSnapshot{{
		ID:           "snap_2",
		ResourceType: "athlete_profile_html",
		ResourceKey:  "profile_123",
		SourceURL:    "https://smoothcomp.com/en/profile/profile_123",
		ContentType:  "text/html",
		StatusCode:   200,
		SHA256:       "body_hash_1",
	}}

	firstHash, err := computeSourceSnapshotHash(first)
	if err != nil {
		t.Fatalf("compute first source snapshot hash: %v", err)
	}
	secondHash, err := computeSourceSnapshotHash(second)
	if err != nil {
		t.Fatalf("compute second source snapshot hash: %v", err)
	}
	if firstHash != secondHash {
		t.Fatalf("expected source snapshot hashes to match, got %s and %s", firstHash, secondHash)
	}
}

package audit

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRunnerProducesDeterministicAuditSummary(t *testing.T) {
	dataset, err := LoadDataset(filepath.Join("..", "..", "..", "..", "testdata", "smoothcomp", "audit", "dataset.json"))
	if err != nil {
		t.Fatalf("load dataset: %v", err)
	}

	report, err := NewDefaultRunner().Run(context.Background(), dataset)
	if err != nil {
		t.Fatalf("run audit: %v", err)
	}

	if report.CaseCount != 35 {
		t.Fatalf("unexpected case count: %d", report.CaseCount)
	}
	if report.Summary.ExactMatches != 235 {
		t.Fatalf("unexpected exact match count: %d", report.Summary.ExactMatches)
	}
	if report.Summary.Mismatches != 6 {
		t.Fatalf("unexpected mismatch count: %d", report.Summary.Mismatches)
	}
	if report.Summary.UnsupportedFacts != 16 {
		t.Fatalf("unexpected unsupported fact count: %d", report.Summary.UnsupportedFacts)
	}
	if report.Summary.MissingWarnings != 0 {
		t.Fatalf("unexpected missing warning count: %d", report.Summary.MissingWarnings)
	}

	assertFieldConfidence(t, report, "smoothcomp.event_participants", "person.age", "do not consume yet")
	assertFieldConfidence(t, report, "smoothcomp.athlete_profile_enrichment", "person.full_name", "high confidence")
	assertFieldConfidence(t, report, "smoothcomp.athlete_profile_enrichment", "match.outcome", "high confidence")
	assertFieldConfidence(t, report, "smoothcomp.athlete_profile_enrichment", "match.finish_method", "high confidence")
	assertFieldConfidence(t, report, "smoothcomp.athlete_profile_enrichment", "match_summary.total_wins", "high confidence")
	assertFieldConfidence(t, report, "smoothcomp.event_detail", "event.description", "low confidence")
	assertFieldConfidence(t, report, "smoothcomp.academy_catalog", "organization.country", "do not consume yet")
	assertWarningConfidence(t, report, "smoothcomp.event_detail", "event_info_panels_unavailable", "high confidence")
	assertWarningConfidence(t, report, "smoothcomp.athlete_profile_enrichment", "match_finish_method_missing", "high confidence")

	if !hasMismatch(report, "event_detail_visible_description_not_parsed", "event", "description", CategoryParserDrift) {
		t.Fatalf("expected parser drift mismatch for visible event detail description")
	}
	if !hasMismatch(report, "event_participants_rich_club_id", "person", "age", CategoryNormalizationBug) {
		t.Fatalf("expected normalization bug mismatch for participant person.age")
	}
}

func assertFieldConfidence(t *testing.T, report *Report, pipeline, field, expected string) {
	t.Helper()
	for _, item := range report.FieldReliability {
		if string(item.Pipeline) == pipeline && item.Field == field {
			if item.Confidence != expected {
				t.Fatalf("unexpected confidence for %s %s: %s", pipeline, field, item.Confidence)
			}
			return
		}
	}
	t.Fatalf("field %s %s not found in report", pipeline, field)
}

func assertWarningConfidence(t *testing.T, report *Report, pipeline, code, expected string) {
	t.Helper()
	for _, item := range report.WarningReliability {
		if string(item.Pipeline) == pipeline && item.Code == code {
			if item.Confidence != expected {
				t.Fatalf("unexpected warning confidence for %s %s: %s", pipeline, code, item.Confidence)
			}
			return
		}
	}
	t.Fatalf("warning %s %s not found in report", pipeline, code)
}

func hasMismatch(report *Report, caseID, entityType, field, classification string) bool {
	for _, item := range report.CaseResults {
		if item.ID != caseID {
			continue
		}
		for _, mismatch := range item.Mismatches {
			if mismatch.EntityType == entityType && mismatch.Field == field && mismatch.Classification == classification {
				return true
			}
		}
	}
	return false
}

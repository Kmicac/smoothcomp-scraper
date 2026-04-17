package smoothcomp

import (
	"context"
	"testing"

	"github.com/kmicac/smoothcomp-scraper/internal/core/job"
	platformconfig "github.com/kmicac/smoothcomp-scraper/internal/platform/config"
	"go.uber.org/zap"
)

func TestParseAthleteProfileHTML(t *testing.T) {
	body := mustReadFixture(t, "athletes", "athlete_profile_fixture.html")

	parsed, err := parseAthleteProfileHTML(body, "7788", "https://smoothcomp.com/en/profile/7788", "snap_profile_html")
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	if parsed.Person.FullName != "Maria Silva" {
		t.Fatalf("unexpected full name: %s", parsed.Person.FullName)
	}
	if parsed.Person.BeltRank != "Brown belt" {
		t.Fatalf("unexpected belt rank: %s", parsed.Person.BeltRank)
	}
	if parsed.Person.OrganizationSourceID != "academy_name:alliance-sao-paulo" {
		t.Fatalf("unexpected organization source id: %s", parsed.Person.OrganizationSourceID)
	}
	if parsed.Organization == nil || parsed.Organization.Name != "Alliance Sao Paulo" {
		t.Fatalf("expected organization to be parsed")
	}
	if parsed.Person.Attributes["total_wins"] != "18" {
		t.Fatalf("unexpected total wins: %s", parsed.Person.Attributes["total_wins"])
	}
}

func TestParseAthleteProfileEventsJSON(t *testing.T) {
	body := mustReadFixture(t, "athletes", "athlete_profile_events_fixture.json")

	parsed, nextPageURL, err := parseAthleteProfileEventsJSON(body, "7788", "snap_profile_events")
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if nextPageURL != nil {
		t.Fatalf("expected nil next page url, got %v", *nextPageURL)
	}
	if len(parsed.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(parsed.Events))
	}
	if parsed.Events[1].Status != "scheduled" {
		t.Fatalf("unexpected second event status: %s", parsed.Events[1].Status)
	}
	if parsed.Attributes["competition_total_wins"] != "1" {
		t.Fatalf("unexpected competition_total_wins: %s", parsed.Attributes["competition_total_wins"])
	}
	if parsed.Attributes["competition_total_losses"] != "1" {
		t.Fatalf("unexpected competition_total_losses: %s", parsed.Attributes["competition_total_losses"])
	}
}

func TestParseAthleteProfileEventsJSONExtractsMatches(t *testing.T) {
	body := mustReadFixture(t, "athletes", "athlete_profile_match_history_fixture.json")

	parsed, nextPageURL, err := parseAthleteProfileEventsJSON(body, "7788", "snap_profile_match_history")
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if nextPageURL != nil {
		t.Fatalf("expected nil next page url, got %v", *nextPageURL)
	}
	if len(parsed.Matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(parsed.Matches))
	}
	if parsed.Matches[0].FinishMethod != "submission" {
		t.Fatalf("unexpected first match finish method: %s", parsed.Matches[0].FinishMethod)
	}
	if parsed.Matches[1].OpponentName != "Bruno Lima" {
		t.Fatalf("unexpected second match opponent: %s", parsed.Matches[1].OpponentName)
	}
	if parsed.MatchSummary == nil {
		t.Fatalf("expected match summary")
	}
	if parsed.MatchSummary.TotalWins != 2 || parsed.MatchSummary.TotalLosses != 1 {
		t.Fatalf("unexpected summary totals: %+v", *parsed.MatchSummary)
	}
	if len(parsed.Warnings) != 1 || parsed.Warnings[0].Code != "match_finish_method_missing" {
		t.Fatalf("unexpected warnings: %+v", parsed.Warnings)
	}
}

func TestParseAthleteProfileEventsJSONHandlesPartialMatches(t *testing.T) {
	body := mustReadFixture(t, "athletes", "athlete_profile_match_partial_fixture.json")

	parsed, _, err := parseAthleteProfileEventsJSON(body, "8812", "snap_profile_match_partial")
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(parsed.Matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(parsed.Matches))
	}
	if parsed.Matches[0].Outcome != "walkover" {
		t.Fatalf("unexpected walkover outcome: %s", parsed.Matches[0].Outcome)
	}
	if parsed.MatchSummary == nil || parsed.MatchSummary.TotalMatches != 2 {
		t.Fatalf("unexpected match summary: %+v", parsed.MatchSummary)
	}
	if len(parsed.Warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d", len(parsed.Warnings))
	}
}

func TestAthleteProfilePipelineNormalizeAllowsPartialHistory(t *testing.T) {
	pipeline := NewAthleteProfilePipeline(NewClient(platformconfig.SmoothcompConfig{
		BaseURL: "https://smoothcomp.com",
	}, zap.NewNop()))

	envelope, err := pipeline.Normalize(context.Background(), job.Request{
		Pipeline:      job.PipelineSmoothcompAthleteProfile,
		CorrelationID: "corr_athlete_profile",
		ProfileID:     "7788",
		ProfileURL:    "https://smoothcomp.com/en/profile/7788",
	}, []job.RawSnapshot{
		{
			ID:           "snap_profile_html",
			ResourceType: "athlete_profile_html",
			SourceURL:    "https://smoothcomp.com/en/profile/7788",
			StatusCode:   200,
			Body:         mustReadFixture(t, "athletes", "athlete_profile_fixture.html"),
		},
		{
			ID:           "snap_profile_events",
			ResourceType: "athlete_profile_events_json",
			StatusCode:   503,
			Body:         []byte(`{"message":"temporarily unavailable"}`),
		},
	})
	if err != nil {
		t.Fatalf("normalize fixture: %v", err)
	}

	if len(envelope.People) != 1 {
		t.Fatalf("expected one person, got %d", len(envelope.People))
	}
	if len(envelope.Warnings) != 1 {
		t.Fatalf("expected one warning, got %d", len(envelope.Warnings))
	}
	if envelope.Warnings[0].Code != "athlete_profile_events_unavailable" {
		t.Fatalf("unexpected warning code: %s", envelope.Warnings[0].Code)
	}
}

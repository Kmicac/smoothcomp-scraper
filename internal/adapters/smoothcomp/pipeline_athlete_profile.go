package smoothcomp

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kmicac/smoothcomp-scraper/internal/core/contract"
	coreerrors "github.com/kmicac/smoothcomp-scraper/internal/core/errors"
	"github.com/kmicac/smoothcomp-scraper/internal/core/job"
)

type AthleteProfilePipeline struct {
	client *Client
}

func NewAthleteProfilePipeline(client *Client) *AthleteProfilePipeline {
	return &AthleteProfilePipeline{client: client}
}

func (p *AthleteProfilePipeline) Name() job.Pipeline { return job.PipelineSmoothcompAthleteProfile }
func (p *AthleteProfilePipeline) Provider() string   { return providerName }
func (p *AthleteProfilePipeline) ParserVersion() string {
	return parserVersionAthleteProfile
}
func (p *AthleteProfilePipeline) NormalizationVersion() string {
	return normalizationVersion
}

func (p *AthleteProfilePipeline) Fetch(ctx context.Context, request job.Request) ([]job.RawSnapshot, error) {
	profileURL, err := p.client.BuildProfileURL(request.ProfileID, request.ProfileURL)
	if err != nil {
		return nil, err
	}
	profileID := firstNonEmpty(request.ProfileID, extractIDFromURL(profileURL))
	resp, body, err := p.client.Fetch(ctx, http.MethodGet, profileURL, "text/html,application/xhtml+xml", nil)
	if err != nil {
		return nil, err
	}

	snapshots := []job.RawSnapshot{snapshotForResponse(
		p.ParserVersion(),
		"athlete_profile_html",
		profileID,
		profileURL,
		resp.StatusCode,
		resp.Header.Get("Content-Type"),
		body,
		map[string]string{"profile_id": profileID},
	)}

	nextURL, err := p.client.BuildProfileEventsURL(profileID, "")
	if err != nil {
		return snapshots, nil
	}
	seenURLs := map[string]struct{}{}
	for page := 1; page <= 10 && nextURL != ""; page++ {
		if _, ok := seenURLs[nextURL]; ok {
			break
		}
		seenURLs[nextURL] = struct{}{}
		eventsResp, eventsBody, fetchErr := p.client.FetchOptional(ctx, http.MethodGet, nextURL, "application/json", nil)
		if fetchErr != nil {
			break
		}
		snapshots = append(snapshots, snapshotForResponse(
			p.ParserVersion(),
			"athlete_profile_events_json",
			profileID+"-page-"+itoa(page),
			nextURL,
			eventsResp.StatusCode,
			eventsResp.Header.Get("Content-Type"),
			eventsBody,
			map[string]string{
				"profile_id": profileID,
				"page":       itoa(page),
			},
		))
		if eventsResp.StatusCode < 200 || eventsResp.StatusCode >= 300 {
			break
		}
		_, nextPageURL, parseErr := parseAthleteProfileEventsJSON(eventsBody, profileID, "")
		if parseErr != nil || nextPageURL == nil || *nextPageURL == "" {
			break
		}
		nextURL, err = p.client.BuildProfileEventsURL(profileID, *nextPageURL)
		if err != nil {
			break
		}
	}

	return snapshots, nil
}

func (p *AthleteProfilePipeline) Normalize(_ context.Context, request job.Request, snapshots []job.RawSnapshot) (contract.Envelope, error) {
	if len(snapshots) == 0 {
		return contract.Envelope{}, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.athlete_profile.normalize", true, "no snapshots received", nil)
	}

	var profileSnapshot *job.RawSnapshot
	eventSnapshots := make([]job.RawSnapshot, 0)
	for i := range snapshots {
		switch snapshots[i].ResourceType {
		case "athlete_profile_html":
			profileSnapshot = &snapshots[i]
		case "athlete_profile_events_json":
			eventSnapshots = append(eventSnapshots, snapshots[i])
		}
	}
	if profileSnapshot == nil {
		return contract.Envelope{}, coreerrors.New(coreerrors.CategoryParsing, coreerrors.CodeParseFailed, "smoothcomp.athlete_profile.normalize", true, "missing athlete profile html snapshot", nil)
	}

	profileID := firstNonEmpty(request.ProfileID, extractIDFromURL(profileSnapshot.SourceURL))
	parsed, err := parseAthleteProfileHTML(profileSnapshot.Body, profileID, profileSnapshot.SourceURL, profileSnapshot.ID)
	if err != nil {
		return contract.Envelope{}, err
	}

	warnings := append([]contract.Warning{}, parsed.Warnings...)
	events := make([]contract.Event, 0)
	matches := make([]contract.Match, 0)
	var aggregatedMatchSummary *contract.MatchSummary
	for _, snapshot := range eventSnapshots {
		if snapshot.StatusCode < 200 || snapshot.StatusCode >= 300 {
			warnings = append(warnings, contract.Warning{
				Code:             "athlete_profile_events_unavailable",
				Message:          "athlete profile events endpoint returned non-success status",
				SubjectType:      "person",
				SubjectID:        parsed.Person.SourceID,
				SourceSnapshotID: snapshot.ID,
			})
			continue
		}
		parsedEvents, _, parseErr := parseAthleteProfileEventsJSON(snapshot.Body, parsed.Person.SourceID, snapshot.ID)
		if parseErr != nil {
			warnings = append(warnings, contract.Warning{
				Code:             "athlete_profile_events_parse_failed",
				Message:          parseErr.Error(),
				SubjectType:      "person",
				SubjectID:        parsed.Person.SourceID,
				SourceSnapshotID: snapshot.ID,
			})
			continue
		}
		parsed.Person.Attributes = mergeStringMap(parsed.Person.Attributes, parsedEvents.Attributes)
		events = append(events, parsedEvents.Events...)
		matches = append(matches, parsedEvents.Matches...)
		warnings = append(warnings, parsedEvents.Warnings...)
		if parsedEvents.MatchSummary != nil {
			aggregatedMatchSummary = mergeMatchSummary(aggregatedMatchSummary, parsedEvents.MatchSummary)
		}
	}
	matchSummaries := make([]contract.MatchSummary, 0, 1)
	if aggregatedMatchSummary != nil {
		matchSummaries = append(matchSummaries, *aggregatedMatchSummary)
		warnings = append(warnings, compareProfileSummaryAgainstMatches(parsed.Person, *aggregatedMatchSummary, profileSnapshot.ID)...)
	}

	organizations := make([]contract.Organization, 0, 1)
	if parsed.Organization != nil {
		organizations = append(organizations, *parsed.Organization)
	}

	return contract.Envelope{
		ContractVersion:      contract.CurrentContractVersion,
		Provider:             providerName,
		Pipeline:             string(job.PipelineSmoothcompAthleteProfile),
		CorrelationID:        request.CorrelationID,
		ParserVersion:        p.ParserVersion(),
		NormalizationVersion: p.NormalizationVersion(),
		GeneratedAt:          time.Now().UTC(),
		Scope: contract.Scope{
			ProfileID: parsed.Person.SourceID,
		},
		Events:         dedupeEvents(events),
		Organizations:  organizations,
		People:         []contract.Person{parsed.Person},
		Matches:        dedupeMatches(matches),
		MatchSummaries: matchSummaries,
		Warnings:       warnings,
		Metadata: map[string]string{
			"primary_snapshot_id": profileSnapshot.ID,
		},
	}, nil
}

func (p *AthleteProfilePipeline) Publish(_ context.Context, _ job.Request, normalized contract.Envelope) (contract.Envelope, error) {
	return normalized, nil
}

func compareProfileSummaryAgainstMatches(person contract.Person, summary contract.MatchSummary, snapshotID string) []contract.Warning {
	checks := []struct {
		key      string
		expected int
	}{
		{"total_wins", summary.TotalWins},
		{"total_losses", summary.TotalLosses},
		{"wins_by_submission", summary.WinsBySubmission},
		{"wins_by_points", summary.WinsByPoints},
		{"wins_by_decision", summary.WinsByDecision},
		{"wins_by_dq", summary.WinsByDQ},
		{"losses_by_submission", summary.LossesBySubmission},
		{"losses_by_points", summary.LossesByPoints},
		{"losses_by_decision", summary.LossesByDecision},
		{"losses_by_dq", summary.LossesByDQ},
	}

	warnings := make([]contract.Warning, 0)
	for _, check := range checks {
		raw, ok := person.Attributes[check.key]
		if !ok || strings.TrimSpace(raw) == "" {
			continue
		}
		value, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || value == check.expected {
			continue
		}
		warnings = append(warnings, contract.Warning{
			Code:             "match_summary_inconsistent_with_profile",
			Message:          "derived match summary did not match visible profile counters",
			SubjectType:      "person",
			SubjectID:        person.SourceID,
			SourceSnapshotID: snapshotID,
		})
		break
	}
	return warnings
}

func mergeMatchSummary(existing *contract.MatchSummary, incoming *contract.MatchSummary) *contract.MatchSummary {
	if incoming == nil {
		return existing
	}
	if existing == nil {
		copy := *incoming
		copy.Attributes = mergeStringMap(map[string]string{}, incoming.Attributes)
		copy.RawReferenceIDs = append([]string{}, incoming.RawReferenceIDs...)
		return &copy
	}

	existing.TotalMatches += incoming.TotalMatches
	existing.TotalWins += incoming.TotalWins
	existing.TotalLosses += incoming.TotalLosses
	existing.WinsBySubmission += incoming.WinsBySubmission
	existing.WinsByPoints += incoming.WinsByPoints
	existing.WinsByDecision += incoming.WinsByDecision
	existing.WinsByDQ += incoming.WinsByDQ
	existing.LossesBySubmission += incoming.LossesBySubmission
	existing.LossesByPoints += incoming.LossesByPoints
	existing.LossesByDecision += incoming.LossesByDecision
	existing.LossesByDQ += incoming.LossesByDQ
	existing.Attributes = mergeStringMap(existing.Attributes, incoming.Attributes)
	existing.RawReferenceIDs = append(existing.RawReferenceIDs, incoming.RawReferenceIDs...)
	if existing.Confidence != "low" && incoming.Confidence == "low" {
		existing.Confidence = "low"
	}
	if existing.Confidence == "high" && incoming.Confidence == "medium" {
		existing.Confidence = "medium"
	}
	return existing
}

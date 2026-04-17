package smoothcomp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kmicac/smoothcomp-scraper/internal/core/job"
	platformconfig "github.com/kmicac/smoothcomp-scraper/internal/platform/config"
	"go.uber.org/zap"
)

func TestParseEventDetailHTML(t *testing.T) {
	body := mustReadFixture(t, "event_detail", "event_detail_fixture.html")

	parsed, err := parseEventDetailHTML(body, "25258", "https://smoothcomp.com/en/event/25258", "snap_event_html")
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	if parsed.Event.SourceID != "25258" {
		t.Fatalf("unexpected event id: %s", parsed.Event.SourceID)
	}
	if parsed.Event.Name != "South American Open 2026" {
		t.Fatalf("unexpected event name: %s", parsed.Event.Name)
	}
	if parsed.Event.City != "Buenos Aires" {
		t.Fatalf("unexpected city: %s", parsed.Event.City)
	}
	if parsed.Event.OrganizerName != "Smooth Events Latam" {
		t.Fatalf("unexpected organizer: %s", parsed.Event.OrganizerName)
	}
}

func TestEventDetailPipelineNormalize(t *testing.T) {
	pipeline := NewEventDetailPipeline(NewClient(platformconfig.SmoothcompConfig{
		BaseURL: "https://smoothcomp.com",
	}, zap.NewNop()))

	envelope, err := pipeline.Normalize(context.Background(), job.Request{
		Pipeline:      job.PipelineSmoothcompEventDetail,
		CorrelationID: "corr_event_detail",
		EventID:       "25258",
		EventURL:      "https://smoothcomp.com/en/event/25258",
	}, []job.RawSnapshot{
		{
			ID:           "snap_event_html",
			ResourceType: "event_detail_html",
			SourceURL:    "https://smoothcomp.com/en/event/25258",
			StatusCode:   200,
			Body:         mustReadFixture(t, "event_detail", "event_detail_fixture.html"),
		},
		{
			ID:           "snap_event_panels",
			ResourceType: "event_info_panels_json",
			StatusCode:   200,
			Body:         mustReadFixture(t, "event_detail", "event_info_panels_fixture.json"),
		},
		{
			ID:           "snap_event_cms",
			ResourceType: "event_cms_json",
			StatusCode:   200,
			Body:         mustReadFixture(t, "event_detail", "event_cms_fixture.json"),
		},
	})
	if err != nil {
		t.Fatalf("normalize fixture: %v", err)
	}

	if len(envelope.Events) != 1 {
		t.Fatalf("expected one event, got %d", len(envelope.Events))
	}
	event := envelope.Events[0]
	if event.Attributes["cms_blocks_count"] != "2" {
		t.Fatalf("unexpected cms_blocks_count: %q", event.Attributes["cms_blocks_count"])
	}
	if event.Attributes["registration_open"] != "true" {
		t.Fatalf("unexpected registration_open attribute: %q", event.Attributes["registration_open"])
	}
	if len(envelope.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %d", len(envelope.Warnings))
	}
}

func mustReadFixture(t *testing.T, parts ...string) []byte {
	t.Helper()
	pathParts := append([]string{"..", "..", "..", "testdata", "smoothcomp"}, parts...)
	body, err := os.ReadFile(filepath.Join(pathParts...))
	if err != nil {
		t.Fatalf("read fixture %v: %v", parts, err)
	}
	return body
}

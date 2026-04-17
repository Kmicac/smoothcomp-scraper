package smoothcomp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseEventCatalogHTML(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "smoothcomp", "events", "past_events_fixture.html")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	events, err := parseEventCatalogHTML("https://smoothcomp.com", "past", body, "snap_test")
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].SourceID != "12345" {
		t.Fatalf("unexpected first event id: %s", events[0].SourceID)
	}
	if events[0].Status != "completed" {
		t.Fatalf("unexpected event status: %s", events[0].Status)
	}
}

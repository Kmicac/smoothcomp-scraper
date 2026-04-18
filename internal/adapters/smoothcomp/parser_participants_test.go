package smoothcomp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseParticipantsJSON(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "smoothcomp", "participants", "event_participants_fixture.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	event, organizations, people, registrations, err := parseParticipantsJSON(body, "25258", "Fixture Event", "https://smoothcomp.com/en/event/25258", "snap_test")
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if event.SourceID != "25258" {
		t.Fatalf("unexpected event id: %s", event.SourceID)
	}
	if len(organizations) != 1 {
		t.Fatalf("expected 1 organization, got %d", len(organizations))
	}
	if len(people) != 2 {
		t.Fatalf("expected 2 people, got %d", len(people))
	}
	if len(registrations) != 2 {
		t.Fatalf("expected 2 registrations, got %d", len(registrations))
	}
	if registrations[0].Division != "Men" {
		t.Fatalf("unexpected division: %s", registrations[0].Division)
	}

	var juanAge *int
	for _, person := range people {
		if person.SourceID == "athlete:5001" {
			juanAge = person.Age
			break
		}
	}
	if juanAge == nil || *juanAge != 28 {
		t.Fatalf("expected normalized participant age 28, got %v", juanAge)
	}
}

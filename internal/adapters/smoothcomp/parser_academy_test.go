package smoothcomp

import (
	"context"
	"testing"

	"github.com/kmicac/smoothcomp-scraper/internal/core/job"
	platformconfig "github.com/kmicac/smoothcomp-scraper/internal/platform/config"
	"go.uber.org/zap"
)

func TestParseAcademyCatalogHTML(t *testing.T) {
	body := mustReadFixture(t, "academies", "academy_catalog_fixture.html")

	items, err := parseAcademyCatalogHTML("https://smoothcomp.com", body, "snap_academy_catalog")
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 academies, got %d", len(items))
	}
	if items[0].SourceID != "111" {
		t.Fatalf("unexpected first academy id: %s", items[0].SourceID)
	}
	if items[1].Name != "Blue Wave JJ" {
		t.Fatalf("unexpected second academy name: %s", items[1].Name)
	}
}

func TestParseAcademyDetailHTML(t *testing.T) {
	body := mustReadFixture(t, "academies", "academy_detail_fixture.html")

	org, err := parseAcademyDetailHTML(body, "https://smoothcomp.com/en/club/111/red-shield", "111", "AR", "snap_academy_detail")
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if org.Name != "Red Shield Academy" {
		t.Fatalf("unexpected academy name: %s", org.Name)
	}
	if org.Attributes["athlete_count"] != "38" {
		t.Fatalf("unexpected athlete_count: %s", org.Attributes["athlete_count"])
	}
	if org.Attributes["instagram_url"] == "" {
		t.Fatalf("expected instagram url")
	}
}

func TestParseAcademyDetailHTMLExtractsCountryFromVisibleLocation(t *testing.T) {
	body := mustReadFixture(t, "audit", "fixtures", "academies", "academy_detail_country_visible_fixture.html")

	org, err := parseAcademyDetailHTML(body, "https://smoothcomp.com/en/club/9104/madrid-roll", "9104", "ES", "snap_academy_detail_country")
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if org.Country != "Spain" {
		t.Fatalf("unexpected academy country: %s", org.Country)
	}
}

func TestAcademyCatalogPipelineNormalizeWarnsOnMissingDetail(t *testing.T) {
	pipeline := NewAcademyCatalogPipeline(NewClient(platformconfig.SmoothcompConfig{
		BaseURL: "https://smoothcomp.com",
	}, zap.NewNop()))

	envelope, err := pipeline.Normalize(context.Background(), job.Request{
		Pipeline:      job.PipelineSmoothcompAcademyCatalog,
		CorrelationID: "corr_academy_catalog",
		Country:       "AR",
	}, []job.RawSnapshot{
		{
			ID:           "snap_academy_catalog",
			ResourceType: "academy_catalog_html",
			StatusCode:   200,
			Body:         mustReadFixture(t, "academies", "academy_catalog_fixture.html"),
		},
		{
			ID:           "snap_academy_detail",
			ResourceType: "academy_detail_html",
			ResourceKey:  "111",
			StatusCode:   200,
			Body:         mustReadFixture(t, "academies", "academy_detail_fixture.html"),
		},
	})
	if err != nil {
		t.Fatalf("normalize fixture: %v", err)
	}

	if len(envelope.Organizations) != 2 {
		t.Fatalf("expected 2 organizations, got %d", len(envelope.Organizations))
	}
	if len(envelope.Warnings) != 1 {
		t.Fatalf("expected one warning, got %d", len(envelope.Warnings))
	}
	if envelope.Warnings[0].Code != "academy_detail_missing" {
		t.Fatalf("unexpected warning code: %s", envelope.Warnings[0].Code)
	}
}

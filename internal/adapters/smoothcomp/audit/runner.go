package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kmicac/smoothcomp-scraper/internal/adapters/smoothcomp"
	"github.com/kmicac/smoothcomp-scraper/internal/core/contract"
	"github.com/kmicac/smoothcomp-scraper/internal/core/job"
	"github.com/kmicac/smoothcomp-scraper/internal/core/port"
	platformconfig "github.com/kmicac/smoothcomp-scraper/internal/platform/config"
	"go.uber.org/zap"
)

type Runner struct {
	pipelines map[job.Pipeline]port.Pipeline
}

func NewRunner(pipelines ...port.Pipeline) *Runner {
	items := make(map[job.Pipeline]port.Pipeline, len(pipelines))
	for _, pipeline := range pipelines {
		items[pipeline.Name()] = pipeline
	}
	return &Runner{pipelines: items}
}

func NewDefaultRunner() *Runner {
	client := smoothcomp.NewClient(platformconfig.SmoothcompConfig{
		BaseURL: "https://smoothcomp.com",
	}, zap.NewNop())
	return NewRunner(
		smoothcomp.NewEventCatalogPipeline(client),
		smoothcomp.NewEventParticipantsPipeline(client),
		smoothcomp.NewEventDetailPipeline(client),
		smoothcomp.NewAthleteProfilePipeline(client),
		smoothcomp.NewAcademyCatalogPipeline(client),
	)
}

func (r *Runner) Run(ctx context.Context, dataset *Dataset) (*Report, error) {
	if dataset == nil {
		return nil, fmt.Errorf("dataset is required")
	}

	report := &Report{
		DatasetName: dataset.Name,
		GeneratedAt: time.Now().UTC(),
		CaseCount:   len(dataset.Cases),
	}

	fieldStats := map[string]*FieldReliability{}
	warningStats := map[string]*WarningReliability{}

	for _, auditCase := range dataset.Cases {
		pipeline, ok := r.pipelines[auditCase.Pipeline]
		if !ok {
			return nil, fmt.Errorf("pipeline %s is not registered", auditCase.Pipeline)
		}

		snapshots, err := loadSnapshots(auditCase.Snapshots)
		if err != nil {
			return nil, fmt.Errorf("load snapshots for %s: %w", auditCase.ID, err)
		}

		envelope, err := pipeline.Normalize(ctx, auditCase.Request, snapshots)
		if err != nil {
			return nil, fmt.Errorf("normalize case %s: %w", auditCase.ID, err)
		}

		caseResult := evaluateCase(auditCase, envelope, fieldStats, warningStats)
		report.CaseResults = append(report.CaseResults, caseResult)
		report.Summary.ExactMatches += caseResult.ExactMatches
		report.Summary.PartialMatches += caseResult.PartialMatches
		report.Summary.UnsupportedFacts += caseResult.UnsupportedFacts
		report.Summary.Mismatches += len(caseResult.Mismatches)
		report.Summary.MissingWarnings += len(caseResult.MissingWarnings)
		report.Summary.UnexpectedWarns += len(caseResult.UnexpectedWarns)
		report.Summary.ExpectedWarnings += len(auditCase.Expectations.Warnings.Required)
	}

	report.FieldReliability = materializeFieldReliability(fieldStats)
	report.WarningReliability = materializeWarningReliability(warningStats)
	sort.Slice(report.CaseResults, func(i, j int) bool { return report.CaseResults[i].ID < report.CaseResults[j].ID })
	return report, nil
}

func loadSnapshots(fixtures []Snapshot) ([]job.RawSnapshot, error) {
	items := make([]job.RawSnapshot, 0, len(fixtures))
	for _, fixture := range fixtures {
		body, err := os.ReadFile(fixture.File)
		if err != nil {
			return nil, err
		}
		items = append(items, job.RawSnapshot{
			ID:           fixture.ID,
			ResourceType: fixture.ResourceType,
			ResourceKey:  fixture.ResourceKey,
			SourceURL:    fixture.SourceURL,
			ContentType:  fixture.ContentType,
			StatusCode:   fixture.StatusCode,
			Body:         body,
			Metadata:     fixture.Metadata,
		})
	}
	return items, nil
}

func evaluateCase(auditCase Case, envelope contract.Envelope, fieldStats map[string]*FieldReliability, warningStats map[string]*WarningReliability) CaseResult {
	result := CaseResult{
		ID:          auditCase.ID,
		Pipeline:    auditCase.Pipeline,
		Description: auditCase.Description,
		Tags:        auditCase.Tags,
	}

	actualWarnings := flattenWarnings(auditCase, envelope.Warnings)
	result.ActualWarnings = actualWarnings

	scopeMap := toJSONMap(envelope.Scope)
	scopeExact, scopePartial := compareFieldMap(auditCase, "scope", "", auditCase.Expectations.Scope, scopeMap, fieldStats, &result)
	result.ExactMatches += scopeExact
	result.PartialMatches += scopePartial
	result.UnsupportedFacts += countUnsupported(auditCase, "scope", "", auditCase.Expectations.Scope, scopeMap, fieldStats)

	compareRecordExpectations(auditCase, "event", auditCase.Expectations.Events, indexEvents(envelope.Events), fieldStats, &result)
	compareRecordExpectations(auditCase, "organization", auditCase.Expectations.Organizations, indexOrganizations(envelope.Organizations), fieldStats, &result)
	compareRecordExpectations(auditCase, "person", auditCase.Expectations.People, indexPeople(envelope.People), fieldStats, &result)
	compareRecordExpectations(auditCase, "registration", auditCase.Expectations.Registrations, indexRegistrations(envelope.Registrations), fieldStats, &result)
	compareRecordExpectations(auditCase, "match", auditCase.Expectations.Matches, indexMatches(envelope.Matches), fieldStats, &result)
	compareRecordExpectations(auditCase, "match_summary", auditCase.Expectations.MatchSummaries, indexMatchSummaries(envelope.MatchSummaries), fieldStats, &result)
	compareWarnings(auditCase, envelope.Warnings, warningStats, &result)

	return result
}

func compareRecordExpectations(
	auditCase Case,
	entityType string,
	expectations map[string]map[string]FieldExpectation,
	records map[string]map[string]any,
	fieldStats map[string]*FieldReliability,
	result *CaseResult,
) {
	for entityID, fields := range expectations {
		record, ok := records[entityID]
		if !ok {
			for field, exp := range fields {
				if exp.Match == MatchSourceNotVisible {
					stats := ensureFieldStats(fieldStats, auditCase.Pipeline, entityType+"."+field)
					stats.UnsupportedCases++
					result.UnsupportedFacts++
					continue
				}
				stats := ensureFieldStats(fieldStats, auditCase.Pipeline, entityType+"."+field)
				stats.VisibleCases++
				result.Mismatches = append(result.Mismatches, Mismatch{
					Pipeline:       auditCase.Pipeline,
					CaseID:         auditCase.ID,
					EntityType:     entityType,
					EntityID:       entityID,
					Field:          field,
					Expected:       exp.Value,
					Classification: firstNonEmpty(exp.MismatchCategory, classifyMissingEntity(auditCase.Tags, field)),
					Notes:          firstNonEmpty(exp.Notes, "expected entity was not present in normalized output"),
				})
				stats.Mismatches++
			}
			continue
		}

		exactMatches, partialMatches := compareFieldMap(auditCase, entityType, entityID, fields, record, fieldStats, result)
		result.ExactMatches += exactMatches
		result.PartialMatches += partialMatches
		result.UnsupportedFacts += countUnsupported(auditCase, entityType, entityID, fields, record, fieldStats)
	}
}

func compareFieldMap(
	auditCase Case,
	entityType string,
	entityID string,
	expectations map[string]FieldExpectation,
	record map[string]any,
	fieldStats map[string]*FieldReliability,
	result *CaseResult,
) (int, int) {
	exactMatches := 0
	partialMatches := 0
	for field, exp := range expectations {
		stats := ensureFieldStats(fieldStats, auditCase.Pipeline, entityType+"."+field)
		switch exp.Match {
		case MatchSourceNotVisible:
			continue
		case MatchExact, MatchPartial:
			stats.VisibleCases++
		default:
			continue
		}

		actual, produced := valueAt(record, field)
		if produced {
			stats.ProducedCases++
		}
		if exp.Match == MatchExact && exactStringMatch(actual, exp.Value) {
			stats.ExactMatches++
			exactMatches++
			continue
		}
		if partialStringMatch(actual, exp.Value) {
			stats.PartialMatches++
			partialMatches++
			continue
		}
		stats.Mismatches++
		result.Mismatches = append(result.Mismatches, Mismatch{
			Pipeline:       auditCase.Pipeline,
			CaseID:         auditCase.ID,
			EntityType:     entityType,
			EntityID:       entityID,
			Field:          field,
			Expected:       exp.Value,
			Actual:         actual,
			Classification: firstNonEmpty(exp.MismatchCategory, classifyMismatch(auditCase.Tags, field, actual, exp.Value)),
			Notes:          exp.Notes,
		})
	}
	return exactMatches, partialMatches
}

func countUnsupported(
	auditCase Case,
	entityType string,
	entityID string,
	expectations map[string]FieldExpectation,
	record map[string]any,
	fieldStats map[string]*FieldReliability,
) int {
	count := 0
	for field, exp := range expectations {
		if exp.Match != MatchSourceNotVisible {
			continue
		}
		stats := ensureFieldStats(fieldStats, auditCase.Pipeline, entityType+"."+field)
		stats.UnsupportedCases++
		if actual, produced := valueAt(record, field); produced && strings.TrimSpace(actual) != "" {
			stats.RiskNotes = appendIfMissing(stats.RiskNotes, "field was populated even when audit dataset marked it as not visibly present in source")
		}
		count++
	}
	return count
}

func compareWarnings(auditCase Case, warnings []contract.Warning, warningStats map[string]*WarningReliability, result *CaseResult) {
	actual := make([]WarningFinding, 0, len(warnings))
	for _, item := range warnings {
		actual = append(actual, WarningFinding{
			Pipeline:    auditCase.Pipeline,
			CaseID:      auditCase.ID,
			Code:        item.Code,
			SubjectType: item.SubjectType,
			SubjectID:   item.SubjectID,
		})
	}

	for _, expected := range auditCase.Expectations.Warnings.Required {
		stats := ensureWarningStats(warningStats, auditCase.Pipeline, expected.Code)
		stats.Expected++
		if containsWarning(warnings, expected) {
			stats.Matched++
			continue
		}
		stats.Missing++
		result.MissingWarnings = append(result.MissingWarnings, WarningFinding{
			Pipeline:       auditCase.Pipeline,
			CaseID:         auditCase.ID,
			Code:           expected.Code,
			SubjectType:    expected.SubjectType,
			SubjectID:      expected.SubjectID,
			Classification: firstNonEmpty(expected.MismatchCategory, CategoryParserDrift),
			Notes:          firstNonEmpty(expected.Notes, "expected warning was not emitted"),
		})
	}

	for _, forbidden := range auditCase.Expectations.Warnings.Forbidden {
		stats := ensureWarningStats(warningStats, auditCase.Pipeline, forbidden.Code)
		if containsWarning(warnings, forbidden) {
			stats.Unexpected++
			result.UnexpectedWarns = append(result.UnexpectedWarns, WarningFinding{
				Pipeline:       auditCase.Pipeline,
				CaseID:         auditCase.ID,
				Code:           forbidden.Code,
				SubjectType:    forbidden.SubjectType,
				SubjectID:      forbidden.SubjectID,
				Classification: firstNonEmpty(forbidden.MismatchCategory, CategoryNormalizationBug),
				Notes:          firstNonEmpty(forbidden.Notes, "warning should not have been emitted for this case"),
			})
		}
	}
}

func containsWarning(actual []contract.Warning, expected WarningExpectation) bool {
	for _, item := range actual {
		if item.Code != expected.Code {
			continue
		}
		if expected.SubjectType != "" && item.SubjectType != expected.SubjectType {
			continue
		}
		if expected.SubjectID != "" && item.SubjectID != expected.SubjectID {
			continue
		}
		return true
	}
	return false
}

func flattenWarnings(auditCase Case, warnings []contract.Warning) []WarningFinding {
	items := make([]WarningFinding, 0, len(warnings))
	for _, item := range warnings {
		items = append(items, WarningFinding{
			Pipeline:    auditCase.Pipeline,
			CaseID:      auditCase.ID,
			Code:        item.Code,
			SubjectType: item.SubjectType,
			SubjectID:   item.SubjectID,
		})
	}
	return items
}

func materializeFieldReliability(items map[string]*FieldReliability) []FieldReliability {
	output := make([]FieldReliability, 0, len(items))
	for _, item := range items {
		if item.VisibleCases > 0 {
			item.Coverage = float64(item.ProducedCases) / float64(item.VisibleCases)
			item.ExactRate = float64(item.ExactMatches) / float64(item.VisibleCases)
			item.PartialRate = float64(item.PartialMatches) / float64(item.VisibleCases)
		}
		item.Confidence = confidenceForField(*item)
		sort.Strings(item.RiskNotes)
		output = append(output, *item)
	}
	sort.Slice(output, func(i, j int) bool {
		if output[i].Pipeline == output[j].Pipeline {
			return output[i].Field < output[j].Field
		}
		return output[i].Pipeline < output[j].Pipeline
	})
	return output
}

func materializeWarningReliability(items map[string]*WarningReliability) []WarningReliability {
	output := make([]WarningReliability, 0, len(items))
	for _, item := range items {
		item.Confidence = confidenceForWarnings(*item)
		sort.Strings(item.RiskNotes)
		output = append(output, *item)
	}
	sort.Slice(output, func(i, j int) bool {
		if output[i].Pipeline == output[j].Pipeline {
			return output[i].Code < output[j].Code
		}
		return output[i].Pipeline < output[j].Pipeline
	})
	return output
}

func ensureFieldStats(items map[string]*FieldReliability, pipeline job.Pipeline, field string) *FieldReliability {
	key := string(pipeline) + "|" + field
	if _, ok := items[key]; !ok {
		items[key] = &FieldReliability{
			Pipeline: pipeline,
			Field:    field,
		}
	}
	return items[key]
}

func ensureWarningStats(items map[string]*WarningReliability, pipeline job.Pipeline, code string) *WarningReliability {
	key := string(pipeline) + "|" + code
	if _, ok := items[key]; !ok {
		items[key] = &WarningReliability{
			Pipeline: pipeline,
			Code:     code,
		}
	}
	return items[key]
}

func confidenceForField(item FieldReliability) string {
	if item.VisibleCases == 0 && item.UnsupportedCases > 0 {
		return "do not consume yet"
	}
	switch {
	case item.VisibleCases >= 3 && item.ExactRate >= 0.95 && item.Coverage >= 0.95 && item.Mismatches == 0:
		return "high confidence"
	case item.VisibleCases >= 3 && item.ExactRate >= 0.75 && item.Coverage >= 0.75:
		return "medium confidence"
	case item.VisibleCases > 0 && item.ExactRate >= 0.50:
		return "low confidence"
	default:
		return "do not consume yet"
	}
}

func confidenceForWarnings(item WarningReliability) string {
	if item.Expected == 0 && item.Unexpected == 0 {
		return "high confidence"
	}
	if item.Expected > 0 && item.Missing == 0 && item.Unexpected == 0 {
		return "high confidence"
	}
	if item.Expected > 0 && item.Missing <= item.Expected/4 && item.Unexpected == 0 {
		return "medium confidence"
	}
	if item.Expected > 0 && item.Matched > 0 {
		return "low confidence"
	}
	return "do not consume yet"
}

func classifyMissingEntity(tags []string, field string) string {
	switch {
	case strings.Contains(field, "source_id"), strings.Contains(field, "source_url"):
		return CategoryIDResolutionBug
	case hasTag(tags, "unsupported-variant"):
		return CategoryUnsupported
	default:
		return CategoryParserDrift
	}
}

func classifyMismatch(tags []string, field, actual, expected string) string {
	switch {
	case strings.Contains(field, "source_id"), strings.Contains(field, "source_url"):
		if hasTag(tags, "subdomain") {
			return CategorySubdomainVariant
		}
		return CategoryIDResolutionBug
	case strings.TrimSpace(actual) == "":
		if hasTag(tags, "partial-source") {
			return CategoryPartialSource
		}
		if hasTag(tags, "unsupported-variant") {
			return CategoryUnsupported
		}
		return CategoryParserDrift
	case hasTag(tags, "subdomain"):
		return CategorySubdomainVariant
	default:
		return CategoryNormalizationBug
	}
}

func indexEvents(items []contract.Event) map[string]map[string]any {
	return indexRecords(items, func(item contract.Event) string { return item.SourceID })
}

func indexOrganizations(items []contract.Organization) map[string]map[string]any {
	return indexRecords(items, func(item contract.Organization) string { return item.SourceID })
}

func indexPeople(items []contract.Person) map[string]map[string]any {
	return indexRecords(items, func(item contract.Person) string { return item.SourceID })
}

func indexRegistrations(items []contract.Registration) map[string]map[string]any {
	return indexRecords(items, func(item contract.Registration) string { return item.SourceID })
}

func indexMatches(items []contract.Match) map[string]map[string]any {
	return indexRecords(items, func(item contract.Match) string { return item.SourceID })
}

func indexMatchSummaries(items []contract.MatchSummary) map[string]map[string]any {
	return indexRecords(items, func(item contract.MatchSummary) string { return item.AthleteSourceID })
}

func indexRecords[T any](items []T, keyFn func(T) string) map[string]map[string]any {
	index := make(map[string]map[string]any, len(items))
	for _, item := range items {
		index[keyFn(item)] = toJSONMap(item)
	}
	return index
}

func toJSONMap(value any) map[string]any {
	body, _ := json.Marshal(value)
	var mapped map[string]any
	_ = json.Unmarshal(body, &mapped)
	return mapped
}

func valueAt(record map[string]any, path string) (string, bool) {
	segments := strings.Split(path, ".")
	var current any = record
	for _, segment := range segments {
		mapped, ok := current.(map[string]any)
		if !ok {
			return "", false
		}
		current, ok = mapped[segment]
		if !ok {
			return "", false
		}
	}
	return stringify(current)
}

func stringify(value any) (string, bool) {
	switch typed := value.(type) {
	case nil:
		return "", false
	case string:
		return typed, strings.TrimSpace(typed) != ""
	case bool:
		return strconv.FormatBool(typed), true
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10), true
		}
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case []any:
		if len(typed) == 0 {
			return "", false
		}
		body, _ := json.Marshal(typed)
		return string(body), true
	default:
		return fmt.Sprint(typed), true
	}
}

func exactStringMatch(actual, expected string) bool {
	return strings.TrimSpace(actual) == strings.TrimSpace(expected)
}

func partialStringMatch(actual, expected string) bool {
	left := normalizeText(actual)
	right := normalizeText(expected)
	if left == "" || right == "" {
		return false
	}
	return left == right || strings.Contains(left, right) || strings.Contains(right, left)
}

func normalizeText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func hasTag(tags []string, target string) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}

func appendIfMissing(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

func (r *Report) Markdown() string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "# Smoothcomp Extraction Audit\n\n")
	fmt.Fprintf(&buf, "Dataset: `%s`\n\n", r.DatasetName)
	fmt.Fprintf(&buf, "Generated at: `%s`\n\n", r.GeneratedAt.Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(&buf, "Cases: `%d`\n\n", r.CaseCount)
	fmt.Fprintf(&buf, "Summary:\n")
	fmt.Fprintf(&buf, "- exact matches: `%d`\n", r.Summary.ExactMatches)
	fmt.Fprintf(&buf, "- partial matches: `%d`\n", r.Summary.PartialMatches)
	fmt.Fprintf(&buf, "- mismatches: `%d`\n", r.Summary.Mismatches)
	fmt.Fprintf(&buf, "- source-not-visible facts: `%d`\n", r.Summary.UnsupportedFacts)
	fmt.Fprintf(&buf, "- expected warnings: `%d`\n", r.Summary.ExpectedWarnings)
	fmt.Fprintf(&buf, "- missing warnings: `%d`\n", r.Summary.MissingWarnings)
	fmt.Fprintf(&buf, "- unexpected warnings: `%d`\n\n", r.Summary.UnexpectedWarns)

	fmt.Fprintf(&buf, "## Field Reliability\n\n")
	for _, item := range r.FieldReliability {
		fmt.Fprintf(&buf, "- `%s` `%s`: confidence `%s`, coverage `%.2f`, exact `%.2f`, partial `%.2f`, mismatches `%d`, unsupported `%d`\n",
			item.Pipeline, item.Field, item.Confidence, item.Coverage, item.ExactRate, item.PartialRate, item.Mismatches, item.UnsupportedCases)
	}

	fmt.Fprintf(&buf, "\n## Warning Reliability\n\n")
	for _, item := range r.WarningReliability {
		fmt.Fprintf(&buf, "- `%s` `%s`: confidence `%s`, expected `%d`, matched `%d`, missing `%d`, unexpected `%d`\n",
			item.Pipeline, item.Code, item.Confidence, item.Expected, item.Matched, item.Missing, item.Unexpected)
	}

	fmt.Fprintf(&buf, "\n## Findings By Case\n\n")
	for _, item := range r.CaseResults {
		if len(item.Mismatches) == 0 && len(item.MissingWarnings) == 0 && len(item.UnexpectedWarns) == 0 {
			continue
		}
		fmt.Fprintf(&buf, "### `%s`\n\n", item.ID)
		fmt.Fprintf(&buf, "- pipeline: `%s`\n", item.Pipeline)
		fmt.Fprintf(&buf, "- description: %s\n", item.Description)
		for _, mismatch := range item.Mismatches {
			fmt.Fprintf(&buf, "- mismatch `%s.%s` (`%s`): expected `%s`, actual `%s`\n",
				mismatch.EntityType, mismatch.Field, mismatch.Classification, mismatch.Expected, mismatch.Actual)
		}
		for _, warning := range item.MissingWarnings {
			fmt.Fprintf(&buf, "- missing warning `%s` (`%s`)\n", warning.Code, warning.Classification)
		}
		for _, warning := range item.UnexpectedWarns {
			fmt.Fprintf(&buf, "- unexpected warning `%s` (`%s`)\n", warning.Code, warning.Classification)
		}
		fmt.Fprintf(&buf, "\n")
	}

	return buf.String()
}

func (r *Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

func Recommendations(report *Report) map[string][]FieldReliability {
	items := map[string][]FieldReliability{
		"safe":    {},
		"partial": {},
		"unsafe":  {},
	}
	for _, field := range report.FieldReliability {
		switch field.Confidence {
		case "high confidence":
			items["safe"] = append(items["safe"], field)
		case "medium confidence", "low confidence":
			items["partial"] = append(items["partial"], field)
		default:
			items["unsafe"] = append(items["unsafe"], field)
		}
	}
	for key := range items {
		sort.Slice(items[key], func(i, j int) bool {
			if items[key][i].Pipeline == items[key][j].Pipeline {
				return items[key][i].Field < items[key][j].Field
			}
			return items[key][i].Pipeline < items[key][j].Pipeline
		})
	}
	return items
}

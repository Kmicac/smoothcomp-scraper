package smoothcomp

import (
	"strconv"

	"github.com/kmicac/smoothcomp-scraper/internal/core/contract"
)

func mergeStringMap(target map[string]string, source map[string]string) map[string]string {
	if len(source) == 0 {
		return target
	}
	if target == nil {
		target = map[string]string{}
	}
	for key, value := range source {
		if value == "" {
			continue
		}
		target[key] = value
	}
	return target
}

func dedupeEvents(events []contract.Event) []contract.Event {
	seen := map[string]struct{}{}
	result := make([]contract.Event, 0, len(events))
	for _, item := range events {
		if item.SourceID == "" {
			continue
		}
		if _, ok := seen[item.SourceID]; ok {
			continue
		}
		seen[item.SourceID] = struct{}{}
		result = append(result, item)
	}
	return result
}

func dedupeMatches(matches []contract.Match) []contract.Match {
	seen := map[string]struct{}{}
	result := make([]contract.Match, 0, len(matches))
	for _, item := range matches {
		if item.SourceID == "" {
			continue
		}
		if _, ok := seen[item.SourceID]; ok {
			continue
		}
		seen[item.SourceID] = struct{}{}
		result = append(result, item)
	}
	return result
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

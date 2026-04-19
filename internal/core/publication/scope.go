package publication

import (
	"strings"

	"github.com/kmicac/smoothcomp-scraper/internal/core/contract"
	"github.com/kmicac/smoothcomp-scraper/internal/core/job"
)

func ScopeKey(provider string, pipeline job.Pipeline, scope contract.Scope) string {
	parts := []string{
		"provider=" + strings.TrimSpace(provider),
		"pipeline=" + strings.TrimSpace(string(pipeline)),
		"country=" + strings.TrimSpace(scope.Country),
		"event_type=" + strings.TrimSpace(scope.EventType),
		"event_id=" + strings.TrimSpace(scope.EventID),
		"profile_id=" + strings.TrimSpace(scope.ProfileID),
	}
	return strings.Join(parts, "|")
}

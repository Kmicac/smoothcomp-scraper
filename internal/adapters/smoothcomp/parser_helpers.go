package smoothcomp

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func nestedString(value any, path ...string) string {
	current := value
	for _, segment := range path {
		asMap, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = asMap[segment]
	}
	return stringValue(current)
}

func attrOrEmpty(selection *goquery.Selection, attr string) string {
	if selection == nil || selection.Length() == 0 {
		return ""
	}
	return strings.TrimSpace(selection.AttrOr(attr, ""))
}

package modelquery

import (
	"regexp"
	"strings"
)

var separatorRun = regexp.MustCompile(`[\s_-]+`)

var dateRun = regexp.MustCompile(`\b(?:19|20)\d{2}(?:-\d{2}(?:-\d{2})?)?\b`)

func withoutDate(id string) string {
	if loc := dateRun.FindStringIndex(id); loc != nil {
		return id[:loc[0]] + id[loc[1]:]
	}
	return id
}

func LineageKey(id string) string {
	lower := withoutDate(strings.ToLower(id))
	if loc := versionToken.FindStringIndex(lower); loc != nil {
		lower = lower[:loc[0]] + lower[loc[1]:]
	}
	collapsed := separatorRun.ReplaceAllString(lower, "-")
	return strings.Trim(collapsed, "-")
}

func GroupByLineage(ids []string) map[string][]string {
	out := make(map[string][]string, len(ids))
	for _, id := range ids {
		key := LineageKey(id)
		out[key] = append(out[key], id)
	}
	return out
}

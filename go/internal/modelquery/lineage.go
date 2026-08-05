package modelquery

import (
	"regexp"
	"strings"
)

// separatorRun matches a run of the separator characters model ids use between
// name tokens (space, hyphen, underscore). Collapsed to a single "-" after the
// version token is removed, so "Gemini 3.5 Pro" and a hypothetical
// "gemini-3.5-pro" normalize to the same key.
var separatorRun = regexp.MustCompile(`[\s_-]+`)

// LineageKey returns the version-free identity of a model id: the id
// lowercased with its comparable version token (the SAME token NewestInLineage
// compares — versionToken's first dotted numeric run) removed, separator runs
// collapsed to "-", and leading/trailing separators trimmed.
//
// Two ids share a LineageKey iff they are the same model line at different
// versions and are therefore mutually substitutable. Different keys are
// different capability classes and must NEVER be substituted — this is what
// keeps "Gemini 3.5 Flash" from ever replacing "Gemini 3.1 Pro" just because
// its version number is higher, and "gpt-5.5-mini" from ever standing in for
// "gpt-5.5".
func LineageKey(id string) string {
	lower := strings.ToLower(id)
	if loc := versionToken.FindStringIndex(lower); loc != nil {
		lower = lower[:loc[0]] + lower[loc[1]:]
	}
	collapsed := separatorRun.ReplaceAllString(lower, "-")
	return strings.Trim(collapsed, "-")
}

// GroupByLineage buckets ids by LineageKey, preserving input order within each
// bucket (downstream tie-breaks — NewestInLineage keeping the first-listed id
// on equal versions — depend on that order surviving).
func GroupByLineage(ids []string) map[string][]string {
	out := make(map[string][]string, len(ids))
	for _, id := range ids {
		key := LineageKey(id)
		out[key] = append(out[key], id)
	}
	return out
}

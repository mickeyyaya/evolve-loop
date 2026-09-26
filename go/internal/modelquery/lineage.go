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

// dateRun matches a calendar-year-anchored date-shaped run (YYYY[-MM[-DD]]),
// e.g. "2024-08-06" or "2024" in "gpt-4o-2024-08-06". Anchored on a 4-digit
// year starting with "19" or "20" so it cannot fire on capability-bearing
// digits like ":8b", ":70b", "32b", or a short dotted version like "4.6" /
// "2.5" — none of those contain a 19xx/20xx-shaped run. Stripped in
// LineageKey in ADDITION to the existing versionToken strip so same-line
// dated snapshots (gpt-4o-2024-08-06 / gpt-4o-2024-11-20) share a key.
var dateRun = regexp.MustCompile(`\b(?:19|20)\d{2}(?:-\d{2}(?:-\d{2})?)?\b`)

// withoutDate returns id with its first date-shaped run (dateRun) removed —
// the ONE date strip shared by LineageKey and the NewestInLineage comparator,
// so the bucket key and the compared version can never disagree about which
// digits are a date.
func withoutDate(id string) string {
	if loc := dateRun.FindStringIndex(id); loc != nil {
		return id[:loc[0]] + id[loc[1]:]
	}
	return id
}

// LineageKey returns the version-free identity of a model id: the id
// lowercased with any date-shaped run (dateRun) and its comparable version
// token (the SAME token NewestInLineage compares — versionToken's first
// dotted numeric run) removed, separator runs collapsed to "-", and
// leading/trailing separators trimmed.
//
// Two ids share a LineageKey iff they are the same model line at different
// versions — including different dated snapshots — and are therefore mutually
// substitutable. Different keys are different capability classes and must
// NEVER be substituted — this is what keeps "Gemini 3.5 Flash" from ever
// replacing "Gemini 3.1 Pro" just because its version number is higher, and
// "gpt-5.5-mini" from ever standing in for "gpt-5.5".
func LineageKey(id string) string {
	lower := withoutDate(strings.ToLower(id))
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

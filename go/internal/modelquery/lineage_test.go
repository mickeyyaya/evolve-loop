package modelquery

import (
	"reflect"
	"testing"
)

// TestLineageKey_RealCatalogIDs pins LineageKey against the real id shapes the
// live catalog has carried, including the pairs whose confusion would be a
// capability downgrade (Flash vs Pro, -mini vs base). Two ids share a key iff
// they are the same model line at different versions.
func TestLineageKey_RealCatalogIDs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id, want string
	}{
		{"Gemini 3.5 Flash (Medium)", "gemini-flash-(medium)"},
		{"Gemini 3.1 Pro (High)", "gemini-pro-(high)"},
		{"Gemini 3.5 Pro (High)", "gemini-pro-(high)"},
		{"gpt-5.5", "gpt"},
		{"gpt-5.5-mini", "gpt-mini"},
		{"llama3.1:8b", "llama:8b"},
		{"llama3.3:8b", "llama:8b"},
		{"llama3.3:latest", "llama:latest"},
		{"opus", "opus"},
		{"opus-4.6", "opus"},
		{"sonnet", "sonnet"},
	}
	for _, c := range cases {
		if got := LineageKey(c.id); got != c.want {
			t.Errorf("LineageKey(%q) = %q, want %q", c.id, got, c.want)
		}
	}
}

// TestLineageKey_SeparatesCapabilityClasses is the safety contract stated
// directly: the pairs that must NEVER be substituted for each other get
// different keys, and the pairs that are version-siblings get the same key.
func TestLineageKey_SeparatesCapabilityClasses(t *testing.T) {
	t.Parallel()
	mustDiffer := [][2]string{
		{"Gemini 3.5 Flash (Medium)", "Gemini 3.1 Pro (High)"}, // Flash must not replace Pro
		{"gpt-5.5", "gpt-5.5-mini"},                            // fast tier can't jump to the deep model
		{"llama3.1:8b", "llama3.3:latest"},                     // :8b and :latest are different lines
		{"llama3.1:8b", "llama3.1:70b"},                        // size suffix must survive the date-run strip
		{"qwen2.5-coder:32b", "qwen2.5-coder:70b"},             // size suffix must survive the date-run strip
	}
	for _, p := range mustDiffer {
		if LineageKey(p[0]) == LineageKey(p[1]) {
			t.Errorf("LineageKey(%q) == LineageKey(%q) = %q — capability classes collided", p[0], p[1], LineageKey(p[0]))
		}
	}
	mustMatch := [][2]string{
		{"Gemini 3.1 Pro (High)", "Gemini 3.5 Pro (High)"},
		{"opus", "opus-4.6"},
		{"llama3.1:8b", "llama3.3:8b"},
		// Migrated from the removed TestLineageKey_DatedSnapshotsStayDistinct_KnownLimitation
		// per its own migration note: date-stamped snapshots of the same line now
		// normalize to the same key so PromoteLatest can act on them
		// (lineage-datestamp-normalization).
		{"gpt-4o-2024-08-06", "gpt-4o-2024-11-20"},
	}
	for _, p := range mustMatch {
		if LineageKey(p[0]) != LineageKey(p[1]) {
			t.Errorf("LineageKey(%q)=%q != LineageKey(%q)=%q — version siblings split", p[0], LineageKey(p[0]), p[1], LineageKey(p[1]))
		}
	}
}

// TestGroupByLineage_OrderPreservingBuckets: ids bucket by LineageKey and each
// bucket preserves input order (deterministic downstream tie-breaks depend on
// this — NewestInLineage keeps the first-listed id on ties).
func TestGroupByLineage_OrderPreservingBuckets(t *testing.T) {
	t.Parallel()
	ids := []string{
		"Gemini 3.1 Pro (High)",
		"Gemini 3.5 Flash (Medium)",
		"Gemini 3.5 Pro (High)",
		"Gemini 3.1 Flash (Medium)",
	}
	got := GroupByLineage(ids)
	want := map[string][]string{
		"gemini-pro-(high)":     {"Gemini 3.1 Pro (High)", "Gemini 3.5 Pro (High)"},
		"gemini-flash-(medium)": {"Gemini 3.5 Flash (Medium)", "Gemini 3.1 Flash (Medium)"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GroupByLineage = %#v, want %#v", got, want)
	}
	if len(GroupByLineage(nil)) != 0 {
		t.Error("GroupByLineage(nil) should be empty")
	}
}

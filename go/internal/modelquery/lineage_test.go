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
	}
	for _, p := range mustMatch {
		if LineageKey(p[0]) != LineageKey(p[1]) {
			t.Errorf("LineageKey(%q)=%q != LineageKey(%q)=%q — version siblings split", p[0], LineageKey(p[0]), p[1], LineageKey(p[1]))
		}
	}
}

// TestLineageKey_DatedSnapshotsStayDistinct_KnownLimitation pins a DELIBERATE
// conservative behavior (adversarial-review finding 3): date-stamped snapshot
// ids of the same line ("gpt-4o-2024-08-06" vs "gpt-4o-2024-11-20") keep
// DIFFERENT keys because only the first dotted numeric run is stripped, so
// PromoteLatest is a no-op for them — the classifier's pick is kept, never
// substituted. That is the fail-safe side of the design (an uncertain
// identity must never substitute); the cost is no automatic promotion across
// dated snapshots. None of the four live CLIs report dated ids today; the
// follow-up is queued as lineage-datestamp-normalization. If this test
// starts failing because normalization was implemented, move these cases to
// the mustMatch table with collision review for size/tag suffixes
// (":8b" vs ":70b" contain digits and must NOT be stripped).
func TestLineageKey_DatedSnapshotsStayDistinct_KnownLimitation(t *testing.T) {
	t.Parallel()
	a, b := LineageKey("gpt-4o-2024-08-06"), LineageKey("gpt-4o-2024-11-20")
	if a == b {
		t.Fatalf("dated snapshots now share key %q — promotion across dates is armed; review the collision risk this pin documents before accepting", a)
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

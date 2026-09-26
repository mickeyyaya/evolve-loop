package modelquery

import (
	"reflect"
	"testing"
)

// TestFreshest_ZeroValueIsNewestVersion: the zero FreshnessPolicy is the
// enumerating-CLI default — newest concrete version via NewestInLineage,
// which is composed, not modified (its versioned-beats-unversioned rule is
// untouched).
func TestFreshest_ZeroValueIsNewestVersion(t *testing.T) {
	t.Parallel()
	var p FreshnessPolicy
	if got := p.Freshest([]string{"Gemini 3.1 Pro (High)", "Gemini 3.5 Pro (High)"}); got != "Gemini 3.5 Pro (High)" {
		t.Errorf("Freshest = %q, want the 3.5 Pro", got)
	}
	if got := p.Freshest(nil); got != "" {
		t.Errorf("Freshest(nil) = %q, want empty", got)
	}
}

// TestFreshest_AliasPreferred: an alias-resolving CLI (claude) declares
// PreferAlias — the bare alias is strictly fresher than any concrete id the
// catalog could cache, because the CLI resolves it to the newest release at
// LAUNCH. Caching a concrete "opus-4.6" would freeze the version.
func TestFreshest_AliasPreferred(t *testing.T) {
	t.Parallel()
	p := FreshnessPolicy{PreferAlias: true, AliasIDs: []string{"opus", "sonnet", "haiku"}}
	// Alias beats a concrete versioned sibling in the same lineage.
	if got := p.Freshest([]string{"opus-4.6", "opus"}); got != "opus" {
		t.Errorf("Freshest = %q, want the alias id", got)
	}
	// No alias in the bucket → falls back to newest-version, never returns "".
	if got := p.Freshest([]string{"opus-4.6", "opus-4.10"}); got != "opus-4.10" {
		t.Errorf("Freshest without alias member = %q, want newest concrete", got)
	}
}

// TestPromoteLatest_WithinLineageOnly is the acceptance case from the live
// catalog: promotion upgrades a stale Pro to the newer Pro but can never hand
// the deep tier to a Flash — substitution happens only inside the selected
// model's own lineage bucket. The classifier keeps 100% of the qualitative
// tier decision; Go keeps 100% of the numeric one.
func TestPromoteLatest_WithinLineageOnly(t *testing.T) {
	t.Parallel()
	sel := map[string]string{
		"deep": "Gemini 3.1 Pro (High)",
		"fast": "Gemini 3.1 Flash (Medium)",
	}
	candidates := []string{
		"Gemini 3.1 Pro (High)",
		"Gemini 3.5 Flash (Medium)",
		"Gemini 3.5 Pro (High)",
		"Gemini 3.1 Flash (Medium)",
	}
	got := PromoteLatest(sel, candidates, FreshnessPolicy{})
	want := map[string]string{
		"deep": "Gemini 3.5 Pro (High)",
		"fast": "Gemini 3.5 Flash (Medium)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PromoteLatest = %#v, want %#v", got, want)
	}
	// Purity: the input selection is not mutated.
	if sel["deep"] != "Gemini 3.1 Pro (High)" {
		t.Error("PromoteLatest mutated its input map")
	}
}

// TestPromoteLatest_MiniStaysMini: the -mini line promotes within itself; it
// never jumps to the base line even though the base carries a higher version.
func TestPromoteLatest_MiniStaysMini(t *testing.T) {
	t.Parallel()
	sel := map[string]string{"fast": "gpt-5.5-mini", "deep": "gpt-5.5"}
	candidates := []string{"gpt-5.5", "gpt-5.5-mini", "gpt-5.6-mini"}
	got := PromoteLatest(sel, candidates, FreshnessPolicy{})
	if got["fast"] != "gpt-5.6-mini" {
		t.Errorf("fast = %q, want gpt-5.6-mini", got["fast"])
	}
	if got["deep"] != "gpt-5.5" {
		t.Errorf("deep = %q, want gpt-5.5 (no newer base exists)", got["deep"])
	}
}

// TestPromoteLatest_UnknownSelectionKept: a selected model absent from the
// candidate list (defensive — sanitizeTierMap normally guarantees membership)
// is kept verbatim rather than dropped or crossed into another bucket.
func TestPromoteLatest_UnknownSelectionKept(t *testing.T) {
	t.Parallel()
	sel := map[string]string{"deep": "mystery-model"}
	got := PromoteLatest(sel, []string{"gpt-5.5"}, FreshnessPolicy{})
	if got["deep"] != "mystery-model" {
		t.Errorf("deep = %q, want the original selection kept", got["deep"])
	}
}

// TestPromoteLatest_MixedDatedBucketsNeverDowngrade: stripping the date run
// widens a lineage bucket to hold dated and undated ids of several versions.
// A date must never outrank a version, and a tie with no date on one side
// must keep the selection, in either candidate listing order.
func TestPromoteLatest_MixedDatedBucketsNeverDowngrade(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, sel  string
		candidates []string
		want       string
	}{
		{"higher undated version kept over older dated", "gpt-5", []string{"gpt-4-2024-04-09", "gpt-5"}, "gpt-5"},
		{"higher dated version kept over later date", "gpt-5-2025-01-01", []string{"gpt-4-2025-06-01", "gpt-5-2025-01-01"}, "gpt-5-2025-01-01"},
		{"date alone is not a version", "gpt-5", []string{"gpt-2024-04-09", "gpt-5"}, "gpt-5"},
		{"undated selection kept on a same-version tie", "gpt-4o", []string{"gpt-4o-2024-08-06", "gpt-4o"}, "gpt-4o"},
		{"dated selection kept on a same-version tie", "gpt-4o-2024-08-06", []string{"gpt-4o", "gpt-4o-2024-08-06"}, "gpt-4o-2024-08-06"},
		{"undated selection kept even with two dated siblings", "gpt-4o", []string{"gpt-4o-2024-08-06", "gpt-4o-2024-11-20", "gpt-4o"}, "gpt-4o"},
		{"dated selection promotes past an undated tie", "gpt-4o-2024-08-06", []string{"gpt-4o", "gpt-4o-2024-11-20", "gpt-4o-2024-08-06"}, "gpt-4o-2024-11-20"},
		{"higher version still promotes a dated selection", "gpt-4-2024-04-09", []string{"gpt-4-2024-04-09", "gpt-5"}, "gpt-5"},
		{"equal-version undated tie keeps the selection", "opus-4.0", []string{"opus-4", "opus-4.0"}, "opus-4.0"},
	}
	for _, tc := range cases {
		candidates := append([]string(nil), tc.candidates...)
		got := PromoteLatest(map[string]string{"deep": tc.sel}, candidates, FreshnessPolicy{})
		if got["deep"] != tc.want {
			t.Errorf("%s: PromoteLatest(deep=%q, %q) = %q, want %q", tc.name, tc.sel, tc.candidates, got["deep"], tc.want)
		}
		if !reflect.DeepEqual(candidates, tc.candidates) {
			t.Errorf("%s: PromoteLatest reordered its candidates: %q", tc.name, candidates)
		}
	}
}

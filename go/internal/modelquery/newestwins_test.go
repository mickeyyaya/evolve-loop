package modelquery

import "testing"

// TestNewestInLineage_PicksHighestVersionAcrossFormats pins the D2 "newest-wins"
// examples verbatim from the latest-model-preference inbox spec: within one
// lineage, the numerically-newest id wins regardless of input order.
func TestNewestInLineage_PicksHighestVersionAcrossFormats(t *testing.T) {
	cases := []struct {
		name string
		ids  []string
		want string
	}{
		{"opus ascending", []string{"opus-4.6", "opus-4.8"}, "opus-4.8"},
		{"opus descending (order must not matter)", []string{"opus-4.8", "opus-4.6"}, "opus-4.8"},
		{"gpt", []string{"gpt-5.4", "gpt-5.5"}, "gpt-5.5"},
		{"gemini", []string{"gemini-3.1", "gemini-3.5"}, "gemini-3.5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewestInLineage(tc.ids)
			if got != tc.want {
				t.Errorf("NewestInLineage(%v) = %q, want %q", tc.ids, got, tc.want)
			}
		})
	}
}

// TestNewestInLineage_RejectsNaiveLexicographicOrdering is the anti-no-op
// negative case: a naive string sort/compare ranks "4.9" above "4.10" because
// '9' > '1' as characters. A genuine numeric-version comparator must not.
func TestNewestInLineage_RejectsNaiveLexicographicOrdering(t *testing.T) {
	ids := []string{"opus-4.9", "opus-4.10"}
	got := NewestInLineage(ids)
	if got != "opus-4.10" {
		t.Errorf("NewestInLineage(%v) = %q, want %q (naive lexicographic compare would wrongly pick opus-4.9)", ids, got, "opus-4.10")
	}
}

// TestNewestInLineage_IgnoresMiniSuffixWhenComparingVersions checks the -mini
// suffix (a capability variant marker, not a version token) does not break
// version extraction, and the full id string (including -mini) is returned
// unmodified.
func TestNewestInLineage_IgnoresMiniSuffixWhenComparingVersions(t *testing.T) {
	ids := []string{"gpt-5.4-mini", "gpt-5.5-mini"}
	got := NewestInLineage(ids)
	if got != "gpt-5.5-mini" {
		t.Errorf("NewestInLineage(%v) = %q, want %q", ids, got, "gpt-5.5-mini")
	}
}

// TestNewestInLineage_EffortParentheticalTieFallsBackToInputOrder checks that
// "(High)"/"(Thinking)" effort-variant parentheticals are ignored for version
// extraction; when the resulting versions tie, the first input id wins
// (deterministic fallback to classifier/original order, never a crash).
func TestNewestInLineage_EffortParentheticalTieFallsBackToInputOrder(t *testing.T) {
	ids := []string{"Gemini 3.1 Pro (High)", "Gemini 3.1 Pro (Thinking)"}
	got := NewestInLineage(ids)
	if got != "Gemini 3.1 Pro (High)" {
		t.Errorf("NewestInLineage(%v) = %q, want %q (tie should keep first-listed id)", ids, got, "Gemini 3.1 Pro (High)")
	}
}

// TestNewestInLineage_UnversionedFallsBackAndNeverCrashes covers the OOD edge:
// a versionless id never outranks a versioned one, and an all-unversioned or
// empty input degrades gracefully to the classifier/original order instead of
// panicking.
func TestNewestInLineage_UnversionedFallsBackAndNeverCrashes(t *testing.T) {
	if got := NewestInLineage([]string{"latest", "gpt-5.5"}); got != "gpt-5.5" {
		t.Errorf("versioned id should beat unversioned: got %q, want %q", got, "gpt-5.5")
	}
	if got := NewestInLineage([]string{"latest", "stable"}); got != "latest" {
		t.Errorf("all-unversioned should fall back to first-listed: got %q, want %q", got, "latest")
	}
	if got := NewestInLineage(nil); got != "" {
		t.Errorf("nil input should return empty string without panic, got %q", got)
	}
	if got := NewestInLineage([]string{"gpt-5.5"}); got != "gpt-5.5" {
		t.Errorf("single-element input should return that element, got %q", got)
	}
}

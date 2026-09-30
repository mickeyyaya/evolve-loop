package modelquery

import "testing"

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

func TestNewestInLineage_RejectsNaiveLexicographicOrdering(t *testing.T) {
	ids := []string{"opus-4.9", "opus-4.10"}
	got := NewestInLineage(ids)
	if got != "opus-4.10" {
		t.Errorf("NewestInLineage(%v) = %q, want %q (naive lexicographic compare would wrongly pick opus-4.9)", ids, got, "opus-4.10")
	}
}

func TestNewestInLineage_IgnoresMiniSuffixWhenComparingVersions(t *testing.T) {
	ids := []string{"gpt-5.4-mini", "gpt-5.5-mini"}
	got := NewestInLineage(ids)
	if got != "gpt-5.5-mini" {
		t.Errorf("NewestInLineage(%v) = %q, want %q", ids, got, "gpt-5.5-mini")
	}
}

func TestNewestInLineage_EffortParentheticalTieFallsBackToInputOrder(t *testing.T) {
	ids := []string{"Gemini 3.1 Pro (High)", "Gemini 3.1 Pro (Thinking)"}
	got := NewestInLineage(ids)
	if got != "Gemini 3.1 Pro (High)" {
		t.Errorf("NewestInLineage(%v) = %q, want %q (tie should keep first-listed id)", ids, got, "Gemini 3.1 Pro (High)")
	}
}

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

func TestNewestInLineage_VersionDecidesBeforeDate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ids  []string
		want string
	}{
		{[]string{"gpt-4-2024-04-09", "gpt-5"}, "gpt-5"},
		{[]string{"gpt-4-2025-06-01", "gpt-5-2025-01-01"}, "gpt-5-2025-01-01"},
		{[]string{"gpt-2024-04-09", "gpt-5"}, "gpt-5"},
		{[]string{"gpt-4o-2024-11-20", "gpt-4o-2024-08-06"}, "gpt-4o-2024-11-20"},
		{[]string{"gpt-4o-2024-08-06", "gpt-4o-2024-11-20"}, "gpt-4o-2024-11-20"},
		{[]string{"gpt-4o", "gpt-4o-2024-08-06"}, "gpt-4o"},
		{[]string{"gpt-4o-2024-08-06", "gpt-4o"}, "gpt-4o-2024-08-06"},
		{[]string{"gpt-2024-04-09", "gpt-2024-06-01"}, "gpt-2024-06-01"},
	}
	for _, tc := range cases {
		if got := NewestInLineage(tc.ids); got != tc.want {
			t.Errorf("NewestInLineage(%q) = %q, want %q", tc.ids, got, tc.want)
		}
	}
}

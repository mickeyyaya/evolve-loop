package clihealth

import (
	"testing"
	"time"
)

func TestModelGroup_StripsTheTrailingEffortVariant(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"Gemini 4 Argon (Medium)":    "Gemini 4 Argon",
		"Gemini 4 Argon (High)":      "Gemini 4 Argon",
		"Gemini 3.8 Flash (High)":    "Gemini 3.8 Flash",
		"Claude Opus 4.6 (Thinking)": "Claude Opus 4.6",
		"claude-opus-5-5":            "claude-opus-5-5",
		" (High)":                    "(High)",
		"":                           "",
	}
	for model, want := range cases {
		if got := ModelGroup(model); got != want {
			t.Errorf("ModelGroup(%q) = %q, want %q", model, got, want)
		}
	}
}

func TestEntryKey_ScopedEntriesDoNotCollideWithTheFamily(t *testing.T) {
	t.Parallel()
	if got := (Entry{Family: "agy"}).Key(); got != "agy" {
		t.Errorf("family-wide Key = %q, want agy: legacy files key by family", got)
	}
	if got := (Entry{Family: "agy", Scope: "Gemini 4 Argon"}).Key(); got != "agy@Gemini 4 Argon" {
		t.Errorf("scoped Key = %q, want agy@Gemini 4 Argon", got)
	}
}

func TestBenchScopedWall_CountsStrikesPerScopeAndLeavesTheFamilyAlone(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 17, 2, 17, 0, 0, time.UTC)
	store := NewStore(t.TempDir(), func() time.Time { return now })
	if _, err := store.BenchWall("agy", "rate_limit", "RESOURCE_EXHAUSTED"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := store.BenchScopedWall("agy", "Gemini 4 Argon", QuotaExhaustedPattern, "Individual quota reached. Resets in 2h."); err != nil {
			t.Fatal(err)
		}
	}
	benches, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	scoped, family := benches["agy@Gemini 4 Argon"], benches["agy"]
	if scoped.Scope != "Gemini 4 Argon" || scoped.Strikes != 2 || scoped.Reason != QuotaExhaustedPattern {
		t.Errorf("scoped entry = %+v, want scope Gemini 4 Argon with 2 strikes", scoped)
	}
	if want := now.Add(2*time.Hour + resetMargin); !scoped.BenchedUntil.Equal(want) {
		t.Errorf("scoped BenchedUntil = %s, want %s from the compact reset hint", scoped.BenchedUntil, want)
	}
	if family.Scope != "" || family.Strikes != 1 || family.Reason != "rate_limit" {
		t.Errorf("family entry = %+v, want the untouched family-wide rate_limit bench", family)
	}
}

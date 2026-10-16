package modelcatalog

import (
	"testing"
	"time"
)

func TestBuildFromSnapshots(t *testing.T) {
	fetched := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	snaps := []CLISnapshot{
		{CLI: "claude", Ready: true, TierModels: map[string]string{
			"fast": "haiku", "balanced": "sonnet", "deep": "opus",
		}},
		{CLI: "codex", Ready: true, TierModels: map[string]string{
			"fast": "gpt-5.4-mini", "balanced": "gpt-5.4", "deep": "gpt-5.5",
		}},
		{CLI: "gemini", Ready: false, TierModels: map[string]string{"fast": "g-flash"}},
		{CLI: "empty", Ready: true, TierModels: map[string]string{}},
		{CLI: "partial", Ready: true, TierModels: map[string]string{"balanced": "m-bal"}},
		{CLI: "noise", Ready: true, TierModels: map[string]string{"ultra": "x", "fast": ""}},
		{CLI: "", Ready: true, TierModels: map[string]string{"fast": "y"}},
	}

	cat := BuildFromSnapshots(snaps, fetched)

	if !cat.FetchedAt.Equal(fetched) {
		t.Fatalf("FetchedAt = %v, want %v", cat.FetchedAt, fetched)
	}
	wantCLIs := []string{"claude", "codex", "partial"}
	if len(cat.CLIs) != len(wantCLIs) {
		t.Fatalf("got %d CLIs %v, want %d %v", len(cat.CLIs), keysOf(cat.CLIs), len(wantCLIs), wantCLIs)
	}
	for _, want := range wantCLIs {
		if _, ok := cat.CLIs[want]; !ok {
			t.Fatalf("missing expected CLI %q in %v", want, keysOf(cat.CLIs))
		}
	}

	if m, ok := cat.Lookup("codex", "deep"); !ok || m != "gpt-5.5" {
		t.Fatalf("codex deep = (%q,%v)", m, ok)
	}
	if m, ok := cat.Lookup("partial", "balanced"); !ok || m != "m-bal" {
		t.Fatalf("partial balanced = (%q,%v)", m, ok)
	}
	if _, ok := cat.Lookup("partial", "fast"); ok {
		t.Fatal("partial fast should be a miss")
	}
	if _, ok := cat.CLIs["noise"]; ok {
		t.Fatal("noise CLI should have been excluded (only non-canonical/empty tiers)")
	}
}

func TestBuildFromSnapshotsEmptyInput(t *testing.T) {
	cat := BuildFromSnapshots(nil, time.Now())
	if !cat.Empty() {
		t.Fatalf("nil snapshots should yield empty catalog, got %v", keysOf(cat.CLIs))
	}
	if cat.CLIs == nil {
		t.Fatal("CLIs map should be initialized (non-nil)")
	}
}

func TestMergeFallbacks_PreservesTierFallbacks(t *testing.T) {
	fetched := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)
	prior := Catalog{
		FetchedAt: fetched.Add(-48 * time.Hour),
		CLIs: map[string]CLIEntry{
			"claude": {
				TierModels:    map[string]string{"deep": "stale-model-id"},
				TierFallbacks: map[string][]string{"deep": {"claude-fable-5"}},
				Source:        SourceLive,
			},
			"gone": {TierFallbacks: map[string][]string{"fast": {"x"}}},
		},
	}
	next := BuildFromSnapshots([]CLISnapshot{{
		CLI:        "claude",
		Ready:      true,
		TierModels: map[string]string{"fast": "claude-haiku-4-5-20251001"},
		Source:     SourceLive,
	}}, fetched)

	merged := MergeFallbacks(prior, next)

	if got := merged.CLIs["claude"].TierFallbacks["deep"]; len(got) != 1 || got[0] != "claude-fable-5" {
		t.Fatalf("tier_fallbacks must survive refresh: got %v", got)
	}
	if got := merged.CLIs["claude"].TierModels["deep"]; got != "" {
		t.Fatalf("stale TierModels must not be restored: got %q", got)
	}
	if !merged.FetchedAt.Equal(fetched) {
		t.Fatalf("FetchedAt = %v, want refresh stamp %v", merged.FetchedAt, fetched)
	}
	if m, ok := merged.DispatchModel("claude", "deep"); !ok || m != "claude-fable-5" {
		t.Fatalf("dispatch via preserved chain = (%q,%v), want (claude-fable-5,true)", m, ok)
	}
	if _, found := merged.CLIs["gone"]; found {
		t.Fatal("merge must not resurrect a CLI absent from the fresh catalog")
	}
	merged.CLIs["claude"].TierFallbacks["deep"][0] = "mutated"
	if prior.CLIs["claude"].TierFallbacks["deep"][0] != "claude-fable-5" {
		t.Fatal("MergeFallbacks must copy chains, not share prior's slices")
	}
}

func TestMergeFallbacks_FreshChainWins(t *testing.T) {
	prior := Catalog{CLIs: map[string]CLIEntry{
		"claude": {TierFallbacks: map[string][]string{"deep": {"prior-chain"}}},
	}}
	next := Catalog{CLIs: map[string]CLIEntry{
		"claude": {
			TierModels:    map[string]string{"deep": "claude-fable-5"},
			TierFallbacks: map[string][]string{"deep": {"fresh-chain"}},
		},
	}}
	merged := MergeFallbacks(prior, next)
	if got := merged.CLIs["claude"].TierFallbacks["deep"]; len(got) != 1 || got[0] != "fresh-chain" {
		t.Fatalf("fresh entry's own chain must win: got %v", got)
	}
}

func keysOf(m map[string]CLIEntry) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

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
		// not ready → excluded entirely
		{CLI: "gemini", Ready: false, TierModels: map[string]string{"fast": "g-flash"}},
		// ready but no tier models → excluded (nothing to catalog)
		{CLI: "empty", Ready: true, TierModels: map[string]string{}},
		// partial: only balanced present → included with just that tier
		{CLI: "partial", Ready: true, TierModels: map[string]string{"balanced": "m-bal"}},
		// non-canonical tier + empty model → both dropped, leaving nothing → excluded
		{CLI: "noise", Ready: true, TierModels: map[string]string{"ultra": "x", "fast": ""}},
		// empty CLI name → excluded
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

	// Full tier table preserved.
	if m, ok := cat.Lookup("codex", "deep"); !ok || m != "gpt-5.5" {
		t.Fatalf("codex deep = (%q,%v)", m, ok)
	}
	// Partial: balanced present, fast/deep absent → miss (caller falls back).
	if m, ok := cat.Lookup("partial", "balanced"); !ok || m != "m-bal" {
		t.Fatalf("partial balanced = (%q,%v)", m, ok)
	}
	if _, ok := cat.Lookup("partial", "fast"); ok {
		t.Fatal("partial fast should be a miss")
	}
	// Non-canonical tier never lands in the catalog.
	if _, ok := cat.CLIs["noise"]; ok {
		t.Fatal("noise CLI should have been excluded (only non-canonical/empty tiers)")
	}
}

func TestBuildFromSnapshotsEmptyInput(t *testing.T) {
	cat := BuildFromSnapshots(nil, time.Now())
	if !cat.Empty() {
		t.Fatalf("nil snapshots should yield empty catalog, got %v", keysOf(cat.CLIs))
	}
	// CLIs map is non-nil so Write/marshal emit `{}` not null.
	if cat.CLIs == nil {
		t.Fatal("CLIs map should be initialized (non-nil)")
	}
}

func keysOf(m map[string]CLIEntry) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

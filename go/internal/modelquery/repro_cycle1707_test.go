package modelquery

import "testing"

func TestPromoteLatest_DatedSnapshotPromotion_Repro(t *testing.T) {
	sel := map[string]string{"deep": "gpt-4o-2024-08-06"}
	candidates := []string{"gpt-4o-2024-08-06", "gpt-4o-2024-11-20"}

	got := PromoteLatest(sel, candidates, FreshnessPolicy{})

	want := "gpt-4o-2024-11-20"
	if got["deep"] != want {
		t.Fatalf("PromoteLatest did not promote across dated snapshots: got %q, want %q (bug: LineageKey buckets dated ids separately)", got["deep"], want)
	}
}

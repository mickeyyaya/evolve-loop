package modelquery

import "testing"

// TestPromoteLatest_DatedSnapshotPromotion_Repro reproduces the
// lineage-datestamp-normalization bug (cycle 1707) through the production
// entry point PromoteLatest — the same call latest.go's liveTiers makes
// after Classify, never LineageKey/NewestInLineage directly. A classifier
// pick of an older dated snapshot ("deep": "gpt-4o-2024-08-06"), with a newer
// dated sibling among the CLI-listed candidates ("gpt-4o-2024-11-20"), must
// promote to the newer snapshot.
func TestPromoteLatest_DatedSnapshotPromotion_Repro(t *testing.T) {
	sel := map[string]string{"deep": "gpt-4o-2024-08-06"}
	candidates := []string{"gpt-4o-2024-08-06", "gpt-4o-2024-11-20"}

	got := PromoteLatest(sel, candidates, FreshnessPolicy{})

	want := "gpt-4o-2024-11-20"
	if got["deep"] != want {
		t.Fatalf("PromoteLatest did not promote across dated snapshots: got %q, want %q (bug: LineageKey buckets dated ids separately)", got["deep"], want)
	}
}

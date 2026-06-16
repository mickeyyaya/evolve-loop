package modelcatalog

import (
	"testing"
	"time"
)

// TestBuildFromSnapshots_AllCanonicalTiersKeptNonCanonicalDropped names the
// modelcatalog.CanonicalTiers var and pins the real filter (refresh.go:49):
// every tier in CanonicalTiers survives into the catalog and a tier outside it
// is dropped. Iterating CanonicalTiers in the assertion ties the test to the
// set itself, so the filter and the set cannot silently drift apart.
func TestBuildFromSnapshots_AllCanonicalTiersKeptNonCanonicalDropped(t *testing.T) {
	const nonCanonical = "ultra"
	tm := map[string]string{nonCanonical: "m-ultra"}
	for _, tier := range CanonicalTiers {
		tm[tier] = "m-" + tier
	}
	cat := BuildFromSnapshots([]CLISnapshot{{CLI: "claude", Ready: true, TierModels: tm}}, time.Now())

	for _, tier := range CanonicalTiers {
		if m, ok := cat.Lookup("claude", tier); !ok || m != "m-"+tier {
			t.Errorf("canonical tier %q = (%q,%v), want (%q,true)", tier, m, ok, "m-"+tier)
		}
	}
	if _, ok := cat.Lookup("claude", nonCanonical); ok {
		t.Errorf("non-canonical tier %q must be dropped from the catalog", nonCanonical)
	}
}

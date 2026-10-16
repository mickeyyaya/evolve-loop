package modelcatalog

import (
	"testing"
	"time"
)

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

package modelcatalog

import "testing"

func TestCanonicalTiersIncludesTop(t *testing.T) {
	seen := make(map[string]int, len(CanonicalTiers))
	for _, tier := range CanonicalTiers {
		seen[tier]++
	}

	if seen["top"] != 1 {
		t.Errorf("CanonicalTiers = %v, want exactly one %q entry", CanonicalTiers, "top")
	}
	for _, want := range []string{"fast", "balanced", "deep"} {
		if seen[want] != 1 {
			t.Errorf("CanonicalTiers = %v, want exactly one %q entry (pre-existing tier must survive)", CanonicalTiers, want)
		}
	}
	if len(CanonicalTiers) != 4 {
		t.Errorf("len(CanonicalTiers) = %d, want 4 (no duplicates, no stray tiers)", len(CanonicalTiers))
	}
	if seen["ultra"] != 0 || seen["high"] != 0 {
		t.Errorf("CanonicalTiers = %v must not contain non-canonical tokens like \"high\" (that is an input ALIAS of \"deep\", not its own tier) or \"ultra\"", CanonicalTiers)
	}
}

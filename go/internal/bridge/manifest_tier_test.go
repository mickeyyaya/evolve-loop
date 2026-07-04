package bridge

import "testing"

// TestTranslateV1TierKey_HighAliasesToDeep pins the latest-model-preference
// TIER VOCABULARY design point: "high" is accepted as an input alias of the
// internal "deep" tier (translateV1TierKey is the single source of truth for
// this mapping, shared by the v1-schema shim and the realizer's fallback
// ladder).
func TestTranslateV1TierKey_HighAliasesToDeep(t *testing.T) {
	if got := translateV1TierKey("high"); got != "deep" {
		t.Errorf(`translateV1TierKey("high") = %q, want "deep"`, got)
	}
}

// TestTranslateV1TierKey_TopIsNotConflatedWithHigh is the negative case: "top"
// is its OWN canonical tier (added by TIER VOCABULARY), distinct from "high"
// which is merely an alias of "deep". A translation table that collapses
// "top" into "deep" would silently erase the new frontier tier.
func TestTranslateV1TierKey_TopIsNotConflatedWithHigh(t *testing.T) {
	if got := translateV1TierKey("top"); got != "top" {
		t.Errorf(`translateV1TierKey("top") = %q, want "top" (pass-through — "top" must stay distinct from "deep")`, got)
	}
}

// TestTranslateV1TierKey_PreexistingMappingsUnchanged is the regression edge:
// the legacy 3-entry table and unknown-key pass-through must survive the new
// "high" alias unchanged.
func TestTranslateV1TierKey_PreexistingMappingsUnchanged(t *testing.T) {
	cases := map[string]string{
		"haiku":       "fast",
		"sonnet":      "balanced",
		"opus":        "deep",
		"some-custom": "some-custom",
	}
	for in, want := range cases {
		if got := translateV1TierKey(in); got != want {
			t.Errorf("translateV1TierKey(%q) = %q, want %q", in, got, want)
		}
	}
}

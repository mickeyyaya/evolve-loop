package bridge

import "testing"

// TestMapCodexModel_CanonicalTiers_Regression guards the cycle-378 incident:
// a policy pin storing the canonical tier "deep" (the vocabulary `evolve setup
// apply` mandates — pins store fast|balanced|deep, never a native model id)
// reached the codex driver, whose mapCodexModel only understood the legacy
// aliases haiku/sonnet/opus. "deep" passed through unrecognized → the driver
// logged "unrecognized model 'deep'", omitted -m, and codex exited rc=1,
// spinning the loop. mapCodexModel MUST translate the canonical tiers
// identically to their legacy aliases, while still passing native ids and
// genuinely unknown values through unchanged.
func TestMapCodexModel_CanonicalTiers_Regression(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		// canonical tiers (the regression) — mirror the legacy aliases
		{"fast", "gpt-5.4-mini"},
		{"balanced", "gpt-5.4"},
		{"deep", "gpt-5.5"},
		// legacy aliases must still map (no regression)
		{"haiku", "gpt-5.4-mini"},
		{"sonnet", "gpt-5.4"},
		{"opus", "gpt-5.5"},
		// native ids + genuinely unknown values pass through unchanged
		{"gpt-5.5", "gpt-5.5"},
		{"weird", "weird"},
	} {
		if got := mapCodexModel(tc.in); got != tc.want {
			t.Errorf("mapCodexModel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

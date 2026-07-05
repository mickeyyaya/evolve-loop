// phase_advisor_tier_test.go — cycle-516 task `advisor-tier-vocab-add-top`
// (RED).
//
// modelcatalog/refresh.go added "top" to CanonicalTiers (fast/balanced/deep/
// top — "the frontier tier, default when a profile/advisor is silent"), but
// sanitizeAdvisorTier — the SOLE gate for advisor-emitted tiers
// (phase_advisor.go:919-925) — still hard-codes the old 3-tier switch and
// silently drops "top" to "" (loosening it to accept extra strings would
// violate the driver_agnostic_model_routing invariant, so this test also
// pins that garbage is still rejected).
package core

import "testing"

func TestSanitizeAdvisorTier(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"fast passes through", "fast", "fast"},
		{"balanced passes through", "balanced", "balanced"},
		{"deep passes through", "deep", "deep"},
		{"top passes through (cycle-516 frontier tier)", "top", "top"},
		{"empty stays empty (common no-op)", "", ""},
		{"garbage tier rejected", "ultra-mega-tier", ""},
		{"legacy model alias rejected — advisor emits tiers, not model names", "opus", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sanitizeAdvisorTier(c.in); got != c.want {
				t.Errorf("sanitizeAdvisorTier(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

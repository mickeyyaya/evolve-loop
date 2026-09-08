package bridge

import "testing"

// TestResolveTierModel_CanonicalTiers_Regression guards the cycle-378
// incident: a policy pin storing the canonical tier "deep" (the vocabulary
// `evolve setup apply` mandates — pins store fast|balanced|deep, never a
// native model id) reached the codex driver, whose tier table only understood
// the legacy aliases haiku/sonnet/opus. "deep" passed through unrecognized →
// the driver logged "unrecognized model 'deep'", omitted -m, and codex exited
// rc=1, spinning the loop. The shared ladder MUST translate every canonical
// tier identically to its legacy alias (relational against the family
// manifest — the values themselves are pinned once, in codex_tier_map_test.go),
// while still passing native ids and genuinely unknown values through.
func TestResolveTierModel_CanonicalTiers_Regression(t *testing.T) {
	m := codexFamilyManifest(t)
	for _, pair := range [][2]string{{"fast", "haiku"}, {"balanced", "sonnet"}, {"deep", "opus"}} {
		canonical, legacy := pair[0], pair[1]
		want := m.ModelTierMap[canonical]
		if want == "" {
			t.Fatalf("family manifest declares no %s tier", canonical)
		}
		if got := resolveTierModel(m, canonical); got != want {
			t.Errorf("resolveTierModel(%q) = %q, want the manifest's %q", canonical, got, want)
		}
		if got := resolveTierModel(m, legacy); got != want {
			t.Errorf("resolveTierModel(%q) = %q, want %q (legacy alias ≡ canonical tier)", legacy, got, want)
		}
	}
	for _, native := range []string{m.ModelTierMap["deep"], "gpt-x", "weird"} {
		if got := resolveTierModel(m, native); got != native {
			t.Errorf("resolveTierModel(%q) = %q, want pass-through", native, got)
		}
	}
}

package bridge

import "testing"

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

package policy

import "testing"

// TestCatalogConfig_RefreshStageResolution pins the refresh_stage dial's
// resolution: explicit value wins; absent derives from AutoRefresh so every
// existing deployment keeps its exact behavior (true ⇒ enforce, the live
// write path; false ⇒ off); an unknown value maps to "off" — the closed-
// vocabulary fail-safe every stage dial in this repo uses (merge_gate
// precedent): a typo must disable the write, never silently arm one.
func TestCatalogConfig_RefreshStageResolution(t *testing.T) {
	t.Parallel()
	boolPtr := func(b bool) *bool { return &b }
	cases := []struct {
		name  string
		block *CatalogPolicy
		want  string
	}{
		{"absent block derives enforce from default AutoRefresh", nil, "enforce"},
		{"auto_refresh false derives off", &CatalogPolicy{AutoRefresh: boolPtr(false)}, "off"},
		{"explicit shadow wins over auto_refresh false", &CatalogPolicy{AutoRefresh: boolPtr(false), RefreshStage: "shadow"}, "shadow"},
		{"explicit enforce wins over auto_refresh false", &CatalogPolicy{AutoRefresh: boolPtr(false), RefreshStage: "enforce"}, "enforce"},
		{"explicit off", &CatalogPolicy{RefreshStage: "off"}, "off"},
		{"unknown value fails safe to off", &CatalogPolicy{RefreshStage: "enforced"}, "off"},
	}
	for _, c := range cases {
		p := Policy{Catalog: c.block}
		if got := p.CatalogConfig().RefreshStage; got != c.want {
			t.Errorf("%s: RefreshStage = %q, want %q", c.name, got, c.want)
		}
	}
}

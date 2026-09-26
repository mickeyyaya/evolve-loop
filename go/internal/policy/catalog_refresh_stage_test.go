package policy

import "testing"

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

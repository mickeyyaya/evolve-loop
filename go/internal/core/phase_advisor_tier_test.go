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

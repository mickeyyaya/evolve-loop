package policy_test

// BridgePolicy — operator-writable override directories. Zero value is
// intentional (each subsystem falls back to its canonical .evolve dir), so the
// accessor is a pure passthrough with a nil-guard.

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestBridgeConfig_Resolution(t *testing.T) {
	cases := []struct {
		name string
		pol  policy.Policy
		want policy.BridgePolicy
	}{
		{"absent-is-zero", policy.Policy{}, policy.BridgePolicy{}},
		{"empty-block-is-zero", policy.Policy{Bridge: &policy.BridgePolicy{}}, policy.BridgePolicy{}},
		{"all-fields", policy.Policy{Bridge: &policy.BridgePolicy{ManifestDir: "/m", CatalogDir: "/c", RecipeDir: "/r"}}, policy.BridgePolicy{ManifestDir: "/m", CatalogDir: "/c", RecipeDir: "/r"}},
		{"manifest-only", policy.Policy{Bridge: &policy.BridgePolicy{ManifestDir: "/m"}}, policy.BridgePolicy{ManifestDir: "/m"}},
		{"anthropic-base-url", policy.Policy{Bridge: &policy.BridgePolicy{AnthropicBaseURL: "https://proxy.example.com"}}, policy.BridgePolicy{AnthropicBaseURL: "https://proxy.example.com"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got policy.BridgePolicy = tc.pol.BridgeConfig()
			if got != tc.want {
				t.Errorf("BridgeConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestLoad_BridgeBlock(t *testing.T) {
	cases := []struct {
		name string
		json string
		want policy.BridgePolicy
	}{
		{"full-block", `{"bridge":{"manifest_dir":"/m","catalog_dir":"/c","recipe_dir":"/r"}}`, policy.BridgePolicy{ManifestDir: "/m", CatalogDir: "/c", RecipeDir: "/r"}},
		{"absent-block-is-zero", `{}`, policy.BridgePolicy{}},
		{"partial-block", `{"bridge":{"recipe_dir":"/r"}}`, policy.BridgePolicy{RecipeDir: "/r"}},
		{"anthropic-base-url", `{"bridge":{"anthropic_base_url":"https://proxy.example.com"}}`, policy.BridgePolicy{AnthropicBaseURL: "https://proxy.example.com"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pol, err := policy.Load(writeTempPolicy(t, tc.json))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := pol.BridgeConfig(); got != tc.want {
				t.Errorf("after Load, BridgeConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

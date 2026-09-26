package policy_test

import (
	"reflect"
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
			if !reflect.DeepEqual(got, tc.want) {
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
			if got := pol.BridgeConfig(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("after Load, BridgeConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestBridgePolicy_PhaseArtifactTimeouts(t *testing.T) {
	cases := []struct {
		name  string
		in    map[string]int
		phase string
		want  int
	}{
		{"compiled-agent-label", nil, "retrospective", 900},
		{"compiled-phase-alias", nil, "retro", 900},
		// scout, not build, is the unlisted probe: build/audit/tdd carry compiled budgets.
		{"unlisted-phase-uses-builtin-sentinel", nil, "scout", 0},
		{"operator-adds-entry", map[string]int{"scout": 600}, "scout", 600},
		{"operator-raises-compiled", map[string]int{"retrospective": 1200}, "retrospective", 1200},
		{"operator-lowers-compiled", map[string]int{"audit": 600}, "audit", 600},
		{"zero-override-rejected", map[string]int{"retrospective": 0}, "retrospective", 900},
		{"negative-override-rejected", map[string]int{"retrospective": -5}, "retrospective", 900},
		{"negative-unlisted-never-negative", map[string]int{"scout": -30}, "scout", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := policy.BridgePolicy{PhaseArtifactTimeoutS: tc.in}.PhaseArtifactTimeouts()
			if got[tc.phase] != tc.want {
				t.Errorf("PhaseArtifactTimeouts()[%q] = %d, want %d", tc.phase, got[tc.phase], tc.want)
			}
		})
	}

	bp := policy.BridgePolicy{PhaseArtifactTimeoutS: map[string]int{"scout": 600}}
	if got := bp.PhaseArtifactTimeouts()["retrospective"]; got != 900 {
		t.Errorf("compiled retrospective entry = %d after unrelated override, want 900", got)
	}
	if bp.ArtifactTimeoutS != 0 {
		t.Errorf("global ArtifactTimeoutS = %d, want 0 (per-phase must not bleed into global)", bp.ArtifactTimeoutS)
	}

	first := policy.BridgePolicy{}.PhaseArtifactTimeouts()
	first["retrospective"] = 1
	if second := (policy.BridgePolicy{}).PhaseArtifactTimeouts()["retrospective"]; second != 900 {
		t.Errorf("after caller mutation, fresh resolve = %d, want 900 (resolver must not alias a package map)", second)
	}
}

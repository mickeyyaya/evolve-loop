package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestParseModelRouting_ZeroValueStatic (mr4d AC2, config-driven-not-a-
// Go-literal regression floor): an EMPTY/absent model_routing key in the
// registry resolves to ModelRoutingStatic — the compiled Go zero value, never
// a literal flip. This must hold regardless of what the checked-in
// .evolve policy ships (AC3 flips only the CHECKED-IN file's value).
func TestParseModelRouting_ZeroValueStatic(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, "phase-registry.json")
	reg := `{"schema_version":3,"config":{},"phases":[]}`
	if err := os.WriteFile(regPath, []byte(reg), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	cfg, _ := Load(regPath, map[string]string{})
	if cfg.ModelRouting != ModelRoutingStatic {
		t.Errorf("ModelRouting = %v, want ModelRoutingStatic (absent key ⇒ Go zero value)", cfg.ModelRouting)
	}
}

// TestParseModelRouting_EscapeHatchStaticOff (mr4d AC4): once auto becomes
// the checked-in default (AC1/AC3), an operator must still be able to opt
// back out. Both an explicit "static" and an unrecognized "off" value parse
// to ModelRoutingStatic — the escape hatch is a policy edit, never an env
// var (I7).
func TestParseModelRouting_EscapeHatchStaticOff(t *testing.T) {
	for _, value := range []string{"static", "off"} {
		dir := t.TempDir()
		regPath := filepath.Join(dir, "phase-registry.json")
		reg := `{"schema_version":3,"config":{"model_routing":"` + value + `"},"phases":[]}`
		if err := os.WriteFile(regPath, []byte(reg), 0o644); err != nil {
			t.Fatalf("write registry: %v", err)
		}
		cfg, _ := Load(regPath, map[string]string{})
		if cfg.ModelRouting != ModelRoutingStatic {
			t.Errorf("model_routing=%q => ModelRouting = %v, want ModelRoutingStatic (escape hatch)", value, cfg.ModelRouting)
		}
	}
}

// TestCheckedInPolicyDefaultsModelRoutingAuto (mr4d AC3): loading the repo's
// OWN checked-in docs/architecture/phase-registry.json — the file
// config.Load actually reads at every real call site (cmd_cycle.go,
// phase_verify.go, router/policy.go all build registryPath from
// "docs/architecture/phase-registry.json", never from .evolve/policy.json)
// — must resolve to ModelRoutingAuto once Task C lands.
//
// NOTE (TDD-engineer, cycle-440): the scout report / api-contract / eval
// mr4d-default-model-routing-auto.md all say the default flip belongs in
// ".evolve/policy.json". Reading the actual producer (config.go's
// registryDoc + every registryPath call site) shows model_routing is parsed
// EXCLUSIVELY from docs/architecture/phase-registry.json's `config.model_routing`
// key; .evolve/policy.json is a separate file (policy.Load) that never feeds
// RoutingConfig.ModelRouting. This test targets the file the code actually
// reads (Rule 8: read first, don't invent an API from context) — see
// test-report.md for the full discrepancy note to Builder/Auditor.
func TestCheckedInPolicyDefaultsModelRoutingAuto(t *testing.T) {
	root := acsassert.RepoRoot(t)
	regPath := filepath.Join(root, "docs", "architecture", "phase-registry.json")
	if _, err := os.Stat(regPath); err != nil {
		t.Fatalf("checked-in registry missing at %s: %v", regPath, err)
	}
	cfg, _ := Load(regPath, map[string]string{})
	if cfg.ModelRouting != ModelRoutingAuto {
		t.Errorf("checked-in docs/architecture/phase-registry.json => ModelRouting = %v, want ModelRoutingAuto", cfg.ModelRouting)
	}
}

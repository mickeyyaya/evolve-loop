package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

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

func TestParseModelRouting_EscapeHatchStaticOff(t *testing.T) {
	// "off" is not a model_routing word; it falls back to static.
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

// Despite "Policy" in its name, this reads the checked-in phase registry, the only source of model_routing.
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

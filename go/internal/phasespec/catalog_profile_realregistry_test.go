package phasespec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRealRegistry_CatalogPhaseResolvesToTrackedProfile(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..")
	cat, err := Load(filepath.Join(repoRoot, "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Fatalf("Load real registry: %v", err)
	}
	spineProfiles := map[string]string{"build": "builder", "audit": "auditor", "tdd": "tdd-engineer"}
	for phase, want := range spineProfiles {
		spec, ok := cat.Get(phase)
		if !ok {
			t.Fatalf("spine phase %s missing from the real registry", phase)
		}
		if got := strings.TrimPrefix(spec.AgentName(), "evolve-"); got != want {
			t.Errorf("phase %s: AgentName %q resolves to profile %q, want %q", phase, spec.AgentName(), got, want)
		}
	}
	for _, name := range cat.Names() {
		spec, _ := cat.Get(name)
		if spec.KindOrDefault() != "llm" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			profile := strings.TrimPrefix(spec.AgentName(), "evolve-")
			if _, err := os.Stat(filepath.Join(repoRoot, ".evolve", "profiles", profile+".json")); err != nil {
				t.Errorf("AgentName %q resolves to profile %q with no .evolve/profiles file: %v", spec.AgentName(), profile, err)
			}
			if _, err := os.Stat(filepath.Join(repoRoot, "agents", spec.AgentName()+".md")); err != nil {
				t.Errorf("AgentName %q has no persona agents/%s.md: %v", spec.AgentName(), spec.AgentName(), err)
			}
		})
	}
}

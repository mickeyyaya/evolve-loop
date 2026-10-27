package tdd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestShouldSkip_APolicyMandatoryTDDRunsAndARegistryOffSkipsToBuild(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"mandatory_phases":["tdd"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if skip, _, _, _ := (hooks{}).ShouldSkip(core.PhaseRequest{ProjectRoot: root}); skip {
		t.Error("a tdd in the policy mandatory_phases must run")
	}
	off := t.TempDir()
	if err := os.MkdirAll(filepath.Join(off, "docs", "architecture"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(off, "docs", "architecture", "phase-registry.json"), []byte(`{"phases":[{"name":"tdd","enabled":"off"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	skip, verdict, next, _ := (hooks{}).ShouldSkip(core.PhaseRequest{ProjectRoot: off})
	if !skip || verdict != core.VerdictSKIPPED || next != string(core.PhaseBuild) {
		t.Errorf("ShouldSkip with tdd turned off in the registry = (%v, %q, %q), want a skip to build", skip, verdict, next)
	}
}

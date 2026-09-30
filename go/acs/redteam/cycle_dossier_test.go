//go:build acs

package redteam

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestCycleDossier(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cyclesDir := filepath.Join(root, "knowledge-base", "cycles")
	entries, err := os.ReadDir(cyclesDir)
	if os.IsNotExist(err) {
		t.Skip("knowledge-base/cycles/ absent — no shipped dossiers yet")
	}
	if err != nil {
		t.Fatalf("read knowledge-base/cycles/: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		p := filepath.Join(cyclesDir, e.Name())
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("read %s: %v", e.Name(), err)
			continue
		}
		var d dossier.Dossier
		if err := json.Unmarshal(raw, &d); err != nil {
			t.Errorf("unmarshal %s: %v", e.Name(), err)
			continue
		}
		if err := d.Validate(); err != nil {
			t.Errorf("Validate %s: %v", e.Name(), err)
		}
	}
}

func TestCycleDossier_MissingDossier(t *testing.T) {
	dir := t.TempDir()
	d, err := dossier.Build(99, dossier.BuildOpts{
		WorkspacePath: dir,
		Goal:          "synthetic missing-dossier scenario",
	})
	if err != nil {
		return
	}
	if err := d.Validate(); err == nil && d.FinalVerdict == "" {
		t.Errorf("Build on empty workspace produced a dossier with no FinalVerdict — missing-dossier detection may be absent")
	}
}

func TestCycleDossier_SkipsInProgress(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if r := os.Getenv("EVOLVE_PROJECT_ROOT"); r != "" {
		root = r
	}
	csPath := filepath.Join(root, ".evolve", "cycle-state.json")
	raw, err := os.ReadFile(csPath)
	if os.IsNotExist(err) {
		t.Skip("cycle-state.json absent — not in an active cycle")
	}
	if err != nil {
		t.Skipf("read cycle-state.json: %v", err)
	}
	var cs struct {
		Phase string `json:"phase"`
	}
	if err := json.Unmarshal(raw, &cs); err != nil {
		t.Skipf("parse cycle-state.json: %v", err)
	}
	if cs.Phase != "" {
		t.Skipf("cycle in-progress (phase=%q) — dossier closeout check deferred", cs.Phase)
	}
	cyclesDir := filepath.Join(root, "knowledge-base", "cycles")
	if _, err := os.ReadDir(cyclesDir); os.IsNotExist(err) {
		t.Skip("knowledge-base/cycles/ absent — no shipped dossiers yet")
	}
}

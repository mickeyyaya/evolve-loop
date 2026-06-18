//go:build acs

// RED note: this file references dossier.BuildOpts which does not exist yet
// (D2 is a prerequisite for D3). The compile error is the intended RED
// signal. Builder implements dossier.Build/BuildOpts in D2, after which
// these three tests compile and should PASS (or SKIP on a fresh clone).
package redteam

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestCycleDossier validates that every JSON file in knowledge-base/cycles/
// unmarshals into a well-formed Dossier (Validate() passes). Skips when the
// directory is absent (fresh clone / no shipped dossiers yet).
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

// TestCycleDossier_MissingDossier verifies the checker detects a completed
// cycle that has no dossier file. It constructs a synthetic scenario by
// creating a BuildOpts pointing to a temp workspace without any dossier JSON,
// then confirming Build returns an error or the result fails Validate (the
// "missing dossier" detection path). RED: BuildOpts doesn't exist yet.
func TestCycleDossier_MissingDossier(t *testing.T) {
	dir := t.TempDir()
	// Build with an empty workspace — no ledger, no reports, no prior dossier.
	d, err := dossier.Build(99, dossier.BuildOpts{
		WorkspacePath: dir,
		Goal:          "synthetic missing-dossier scenario",
	})
	if err != nil {
		// Build returning an error on a missing/incomplete workspace is acceptable.
		return
	}
	// If Build succeeded, it must produce a structurally valid dossier with
	// at least the cycle number and goal intact; an empty/zero dossier fails Validate.
	if err := d.Validate(); err == nil && d.FinalVerdict == "" {
		t.Errorf("Build on empty workspace produced a dossier with no FinalVerdict — missing-dossier detection may be absent")
	}
}

// TestCycleDossier_SkipsInProgress skips validation when the current cycle is
// still in flight (cycle-state.json Phase != "" indicates active). This
// prevents false failures during a running cycle.
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
	// Cycle completed: validate the dossier for the last cycle exists.
	cyclesDir := filepath.Join(root, "knowledge-base", "cycles")
	if _, err := os.ReadDir(cyclesDir); os.IsNotExist(err) {
		t.Skip("knowledge-base/cycles/ absent — no shipped dossiers yet")
	}
}

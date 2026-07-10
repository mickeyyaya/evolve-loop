//go:build integration

// apicover_named_test.go — public-API coverage (ADR-0050 Phase 5). Names and
// exercises exported symbols apicover flagged uncovered in this package:
//   - const ExitMissingBin (native.go)
//   - func NewWithDefaultRunnerStage (ship.go)
//   - method Class.IsValid (native.go)
//   - func PluginVersion (statefile.go)
//
// Each test asserts a real contract (Rule 9), not a no-op reference.
package ship

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestClassIsValid pins the documented closed set: the four declared Class
// consts are valid, and unknown / empty strings are not.
func TestClassIsValid(t *testing.T) {
	valid := []Class{ClassCycle, ClassManual, ClassRelease, ClassTrivial}
	for _, c := range valid {
		if !c.IsValid() {
			t.Errorf("Class(%q).IsValid() = false, want true", c)
		}
	}
	for _, c := range []Class{Class(""), Class("bogus")} {
		if c.IsValid() {
			t.Errorf("Class(%q).IsValid() = true, want false", c)
		}
	}
}

// TestNewWithDefaultRunnerStage asserts the stage-aware production constructor
// returns a wired *Phase that satisfies core.PhaseRunner and reports the ship
// phase name.
func TestNewWithDefaultRunnerStage(t *testing.T) {
	p := NewWithDefaultRunnerStage(config.StageEnforce)
	if p == nil {
		t.Fatal("NewWithDefaultRunnerStage returned nil")
	}
	if p.runner == nil {
		t.Error("runner field is nil; want sysexec.DefaultRunner")
	}
	if p.phaseIO != config.StageEnforce {
		t.Errorf("phaseIO = %v, want StageEnforce", p.phaseIO)
	}
	if got, want := p.Name(), string(core.PhaseShip); got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
	// Must satisfy the PhaseRunner contract the orchestrator dispatches against.
	var _ core.PhaseRunner = p
}

// TestPluginVersion_Exported pins the exported resolver's contract: it reads
// .claude-plugin/plugin.json:version and returns "" when the file/key is absent.
// This is the SAME logic the boot self-SHA gate (cmd/evolve) relies on to classify
// a mismatch as within-version, so the two gates never diverge.
func TestPluginVersion_Exported(t *testing.T) {
	root := t.TempDir()
	if got := PluginVersion(root); got != "" {
		t.Errorf("PluginVersion on a repo with no plugin.json = %q, want empty", got)
	}
	dir := filepath.Join(root, ".claude-plugin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"version":"3.2.1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := PluginVersion(root); got != "3.2.1" {
		t.Errorf("PluginVersion = %q, want %q", got, "3.2.1")
	}
}

// TestExitMissingBin pins the documented 127 value for the "required binary
// missing" ship exit code (the conventional shell "command not found" code).
func TestExitMissingBin(t *testing.T) {
	if ExitMissingBin != 127 {
		t.Errorf("ExitMissingBin = %d, want 127", int(ExitMissingBin))
	}
}

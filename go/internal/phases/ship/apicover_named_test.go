//go:build integration

package ship

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

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
	var _ core.PhaseRunner = p
}

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

// ExitMissingBin matches the shell's conventional "command not found" code.
func TestExitMissingBin(t *testing.T) {
	if ExitMissingBin != 127 {
		t.Errorf("ExitMissingBin = %d, want 127", int(ExitMissingBin))
	}
}

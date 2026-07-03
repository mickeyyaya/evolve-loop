package core

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// TestResolveCycleStatePath_FleetOverrideAndDefault names ResolveCycleStatePath
// for apicover and exercises both branches: unset ⇒ the host-global default
// (<evolveDir>/cycle-state.json, byte-identical to the sequential loop); the
// fleet per-run override (ipcenv.CycleStateFileKey) ⇒ that absolute path
// verbatim, so two concurrent lanes never share the host singleton.
func TestResolveCycleStatePath_FleetOverrideAndDefault(t *testing.T) {
	dir := t.TempDir()
	if got, want := ResolveCycleStatePath(dir), filepath.Join(dir, CycleStateFile); got != want {
		t.Errorf("no override: ResolveCycleStatePath(%q) = %q, want host-global %q", dir, got, want)
	}

	override := filepath.Join(dir, "runs", "cycle-7", "cycle-state.json")
	t.Setenv(ipcenv.CycleStateFileKey, override)
	if got := ResolveCycleStatePath(dir); got != override {
		t.Errorf("with fleet override: ResolveCycleStatePath(%q) = %q, want the per-run override %q", dir, got, override)
	}
}

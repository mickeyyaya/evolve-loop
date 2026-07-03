package ipcenv_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// TestIpcenv_ConstValues names all four exported keys in ipcenv for apicover
// and verifies their string values match the IPC protocol.
func TestIpcenv_ConstValues(t *testing.T) {
	if ipcenv.FleetKey != "EVOLVE_FLEET" {
		t.Errorf("FleetKey = %q, want %q", ipcenv.FleetKey, "EVOLVE_FLEET")
	}
	if ipcenv.FleetScopeKey != "EVOLVE_FLEET_SCOPE" {
		t.Errorf("FleetScopeKey = %q, want %q", ipcenv.FleetScopeKey, "EVOLVE_FLEET_SCOPE")
	}
	if ipcenv.WorktreeRootKey != "EVOLVE_WORKTREE_ROOT" {
		t.Errorf("WorktreeRootKey = %q, want %q", ipcenv.WorktreeRootKey, "EVOLVE_WORKTREE_ROOT")
	}
	if ipcenv.CycleStateFileKey != "EVOLVE_CYCLE_STATE_FILE" {
		t.Errorf("CycleStateFileKey = %q, want %q", ipcenv.CycleStateFileKey, "EVOLVE_CYCLE_STATE_FILE")
	}
}

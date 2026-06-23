package ipcenv_test

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/ipcenv"
)

// TestIpcenv_ConstValues names all three exported types in ipcenv for apicover
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
}

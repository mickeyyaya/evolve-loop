package paths

import (
	"path/filepath"
	"testing"
)

// TestRunWorkspace_JoinsTheCycleWorkspace pins the ONE spelling of the
// per-cycle run-workspace layout, <root>/.evolve/runs/cycle-<N> (ADR-0103 unit
// 09): core.RunWorkspacePath projects it and the defect ledger reads through it.
func TestRunWorkspace_JoinsTheCycleWorkspace(t *testing.T) {
	t.Parallel()
	if got, want := RunWorkspace("/root", 7), filepath.Join("/root", ".evolve", "runs", "cycle-7"); got != want {
		t.Fatalf("RunWorkspace = %q, want %q", got, want)
	}
	if got, want := RunWorkspace("rel", 1425), filepath.Join(EvolveDirOf("rel"), "runs", "cycle-1425"); got != want {
		t.Fatalf("RunWorkspace derives from EvolveDirOf: %q, want %q", got, want)
	}
}

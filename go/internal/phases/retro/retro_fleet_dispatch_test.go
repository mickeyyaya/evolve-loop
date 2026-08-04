package retro

// retro_fleet_dispatch_test.go — cycle-1270 Task 2
// (`retro-fleet-worktree-dispatch`), the item's literal acceptance criterion:
// a fleet-mode test proving retro's dispatch carries the LANE worktree.
//
// The root cause is already fixed upstream (worktree-provisioning-retry,
// PR #401 / a497ffe1); what stayed open is the regression test, and the gap it
// must close is a JOIN, not another half. retro_worktree_fallback_test.go
// proves retro mints something; bridge's driver_tmux_repl_workdir_test.go
// proves the guard refuses nothing. Neither proves the value that TRAVELS
// between them satisfies the guard's own predicate.
//
// A fabricated path would satisfy "non-empty" and still strand the lane at the
// guard's isDir() check — which is why this asserts on the directory, not on
// the string.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func TestRetroWorktree_FleetScratchCwdSatisfiesBridgeGuardPredicate(t *testing.T) {
	projectRoot, workspace := t.TempDir(), t.TempDir()
	req := retroFailReq(projectRoot, workspace, "", map[string]string{ipcenv.FleetKey: "1"})

	got := retroWorktree(req)
	if got == "" {
		t.Fatal("retro resolved no worktree under a fleet dispatch with an owned workspace — the bridge then refuses the launch (errWorktreeRequired) and the lane loses its retrospective entirely: a failure in the failure-handler")
	}

	// The bridge guard's predicate, verbatim in substance
	// (driver_tmux_repl.go: `if !isDir(workingDir)` → ExitBadFlags). A value
	// that travels but does not clear this is the same stranded lane with a
	// different error code.
	fi, err := os.Stat(got)
	if err != nil || !fi.IsDir() {
		t.Fatalf("resolved worktree %q is not an existing directory (%v) — the fleet guard rejects it at isDir() and a fabricated path is exactly the shape this must never produce", got, err)
	}

	// The two shapes deliberately not used, re-pinned at the join because this
	// is the value the bridge actually receives.
	if got == projectRoot || strings.HasPrefix(got, projectRoot+string(filepath.Separator)) {
		t.Errorf("resolved worktree %q is inside the shared main tree — worktree is the write-authority predicate (refuted PR #400)", got)
	}
	if cwd, cerr := os.Getwd(); cerr == nil && got == cwd {
		t.Errorf("resolved worktree is the dispatching process cwd (%q) — the exact leak the fleet guard exists to close", got)
	}
	if !strings.HasPrefix(got, workspace+string(filepath.Separator)) {
		t.Errorf("resolved worktree %q is not under the workspace retro owns (%q) — a disposable cwd must live where the lane already has write authority", got, workspace)
	}
}

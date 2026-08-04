package retro

// retro_stale_worktree_test.go — cycle-1278 `retro-fleet-stale-worktree-fallback`,
// the verified-open half of the cycle-1255 CRITICAL that the 1255→1268→1270→1272
// salvage chain progressively narrowed to the EMPTY-worktree shape and then
// declared closed (CHANGELOG, 68322bdf).
//
// The gap: retroWorktree substitutes the scratch cwd only when req.Worktree == "".
// A torn-down fleet lane hands it a NON-EMPTY but STALE path (cs.ActiveWorktree is
// never cleared on lane teardown, cyclerun.go:456/471), which passes through
// verbatim and is then refused by the bridge guard's isDir() check
// (driver_tmux_repl.go:123-126, ExitBadFlags, stderr only) — the lane loses its
// retrospective entirely. A failure in the failure-handler.
//
// The invariant these tests pin is the guard's own predicate, not a string shape:
// under fleet mode retro must never hand the bridge a non-empty path that fails
// isDir(). The three input shapes (empty / existing / stale) are one contract;
// TestRetroWorktree_FleetScratchCwdSatisfiesBridgeGuardPredicate already covers
// the empty one, so these cover the other two plus the non-fleet passthrough that
// an over-broad fix would silently break.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// stalePath returns a path that is guaranteed NOT to exist — the exact shape a
// pruned fleet lane leaves behind in cs.ActiveWorktree.
func stalePath(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "worktrees", "cycle-42824668-9999")
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("fixture path %q must not exist (stat err=%v)", p, err)
	}
	return p
}

// TestRetroWorktree_StaleNonExistentPathFallsBackToScratchCwd is the crux (AC1,
// stale shape). Fleet mode + a torn-down lane's path: retro must fall back to the
// scratch cwd it owns, exactly as it does for the empty shape. Asserting on the
// DIRECTORY (os.Stat) rather than on "not equal to the input" is what makes this
// the guard's predicate: a different fabricated string would still strand the lane.
func TestRetroWorktree_StaleNonExistentPathFallsBackToScratchCwd(t *testing.T) {
	projectRoot, workspace := t.TempDir(), t.TempDir()
	stale := stalePath(t)
	req := retroFailReq(projectRoot, workspace, stale, map[string]string{ipcenv.FleetKey: "1"})

	got := retroWorktree(req)
	if got == stale {
		t.Fatalf("retro passed the torn-down lane's stale worktree %q through verbatim — the bridge guard rejects it at isDir() (ExitBadFlags, stderr only) and the lane loses its retrospective entirely", got)
	}
	if got == "" {
		t.Fatalf("retro resolved no worktree despite owning a workspace (%q) — under fleet mode the bridge then refuses the launch with errWorktreeRequired: the same lost retrospective by a different exit code", workspace)
	}

	// The bridge guard's predicate, verbatim in substance
	// (driver_tmux_repl.go: `if !isDir(workingDir)` → ExitBadFlags).
	fi, err := os.Stat(got)
	if err != nil || !fi.IsDir() {
		t.Fatalf("resolved worktree %q is not an existing directory (%v) — a fabricated path is the exact shape this must never produce", got, err)
	}

	// The two shapes deliberately not used, re-pinned for the stale input.
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

// TestRetroWorktree_FleetNeverEmitsANonExistentPath is the edge axis: with NO
// owned workspace there is nowhere safe to mint, so the fallback yields "" and the
// bridge decides exactly as it does today. What retro may never do is emit a
// non-empty path that fails the guard's isDir() — that is the whole defect class,
// independent of which fallback is available.
func TestRetroWorktree_FleetNeverEmitsANonExistentPath(t *testing.T) {
	stale := stalePath(t)
	req := retroFailReq(t.TempDir(), "", stale, map[string]string{ipcenv.FleetKey: "1"})

	got := retroWorktree(req)
	if got == "" {
		return // nothing to mint, nothing fabricated — the honest degenerate case
	}
	if fi, err := os.Stat(got); err != nil || !fi.IsDir() {
		t.Fatalf("retro emitted the non-existent path %q with no workspace to mint under (input was the stale %q) — every non-empty value retro returns must clear the bridge's isDir() guard", got, stale)
	}
}

// TestRetroWorktree_FleetProvisionedWorktreePassesThroughVerbatim is the
// regression the widened condition must not break (semantic axis). A fallback that
// fires on a LIVE worktree would strand every normal fleet retro in an empty
// scratch dir with no repo — the failure mode the original narrow condition was
// protecting against.
func TestRetroWorktree_FleetProvisionedWorktreePassesThroughVerbatim(t *testing.T) {
	live := t.TempDir() // a real, existing lane worktree
	req := retroFailReq(t.TempDir(), t.TempDir(), live, map[string]string{ipcenv.FleetKey: "1"})

	if got := retroWorktree(req); got != live {
		t.Fatalf("retroWorktree replaced the LIVE lane worktree %q with %q — a fallback that fires on an existing worktree strands every normal fleet retro in a repo-less scratch dir", live, got)
	}
}

// TestRetroWorktree_NonFleetStalePathPassesThroughVerbatim is the negative axis
// on the mode dimension. Outside fleet mode the bridge keeps its process-cwd
// fallback and reports the bad dir loudly to the operator; retro must not
// silently rewrite the operator's designated worktree. Widening the condition
// without keeping it fleet-gated changes single-driver semantics.
func TestRetroWorktree_NonFleetStalePathPassesThroughVerbatim(t *testing.T) {
	stale := stalePath(t)
	req := retroFailReq(t.TempDir(), t.TempDir(), stale, map[string]string{ipcenv.FleetKey: "0"})

	if got := retroWorktree(req); got != stale {
		t.Fatalf("non-fleet dispatch rewrote the operator's designated worktree %q to %q — the fallback exists for the fleet guard's fail-closed window only", stale, got)
	}
}

//go:build acs

// Package cycle1278 materialises the cycle-1278 acceptance criteria for the one
// task triage committed to `## top_n`:
//
//	retro-fleet-stale-worktree-fallback → retroWorktree
//	(go/internal/phases/retro/retro.go:87-92) falls back to the workspace-owned
//	scratch cwd when req.Worktree is empty OR names a directory that does not
//	exist, matching the bridge guard's isDir() predicate
//	(driver_tmux_repl.go:123-126) exactly; and cs.ActiveWorktree is cleared at
//	fleet-lane teardown (go/internal/core/cyclerun.go ~L466-472) once the prune
//	succeeds, so a torn-down lane's path is never handed to the next dispatch.
//
// The deferred task (fix-changelog-false-closure-claim) and the seven dropped
// ids get ZERO predicates — R9.3 floor-binding: predicates bind only to
// triage-committed work.
//
// Predicate-quality note (cycle-85 ban). No predicate here greps source for a
// magic string. 001-003 RUN the acceptance tests in their own packages and
// require an explicit `--- PASS: <name>` line, so a `-run` pattern that matched
// nothing — a deleted or renamed test — cannot green vacuously. 004 takes no
// subprocess at all: it invokes the public bridge helper retro falls back to and
// asserts its return value against the guard's own predicate.
//
// Diversity axes: 001 is the happy path (the two shapes that are RED today);
// 002 is the semantic/regression axis (a fallback that fires unconditionally, or
// leaks outside fleet mode, is the opposite defect); 003 carries its own negative
// case (a PRESERVED worktree must NOT be cleared — resume reclaims the lane by
// that path); 004 is the edge axis on the fallback's own degenerate input.
//
// Flaky-shape compliance: every subprocess is ONE named package with an anchored
// `-run` (measured 0.4-0.5s each at RED, plus compile), `cmd.Dir` is set
// explicitly rather than inherited from the lane's cwd, and there is no
// wall-clock bound, no literal PID, and no `./...` sweep.
package cycle1278

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// runAcceptanceTest executes ONE named test in ONE named package and requires an
// explicit `--- PASS: <name>` line. The explicit-line check is the anti-gaming
// half: `go test -run <nothing-matches>` exits 0, so a deleted or renamed test
// would otherwise green the criterion vacuously.
//
// pkg and name arrive as direct string arguments (not through a struct) so the
// flaky-shape lint's one-hop resolution can see that this package pattern reaches
// a `go test` argv already narrowed by `-run`.
func runAcceptanceTest(t *testing.T, criterion, name, pkg string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		cmd := exec.Command("go", "test", "-count=1", "-v", "-run", "^"+name+"$", pkg)
		cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go") // never inherit the lane's cwd
		out, err := cmd.CombinedOutput()
		text := string(out)
		if err != nil {
			t.Fatalf("%s (%s) is RED — criterion NOT met: %s\n%v\n%s", name, pkg, criterion, err, text)
		}
		if !strings.Contains(text, "--- PASS: "+name) {
			t.Fatalf("%s never ran (no `--- PASS: %s` in %s output) — a deleted or renamed test cannot stand in for the criterion %q:\n%s",
				name, name, pkg, criterion, text)
		}
	})
}

const (
	retroPkg = "./internal/phases/retro"
	corePkg  = "./internal/core"
)

// TestC1278_001_StaleWorktreeFallsBackToScratchCwd is the crux (AC1). Both of
// these are RED before the fix: retroWorktree passes a non-empty stale path
// through verbatim, and the bridge guard then refuses the launch at isDir() —
// the lane loses its retrospective entirely.
func TestC1278_001_StaleWorktreeFallsBackToScratchCwd(t *testing.T) {
	runAcceptanceTest(t,
		"fleet mode + a torn-down lane's stale worktree resolves to an existing scratch dir under the workspace retro owns",
		"TestRetroWorktree_StaleNonExistentPathFallsBackToScratchCwd", retroPkg)
	runAcceptanceTest(t,
		"retro never emits a non-empty path that fails the bridge guard's isDir() check, even with no workspace to mint under",
		"TestRetroWorktree_FleetNeverEmitsANonExistentPath", retroPkg)
}

// TestC1278_002_FallbackStaysScopedToTheGuardsWindow is the semantic axis: the
// widened condition must not become an unconditional fallback. A live lane
// worktree, and any non-fleet dispatch, must still pass through verbatim — and
// the empty shape the 1270 test already pins must stay green, because the three
// input shapes are one contract, not three independent fixes.
func TestC1278_002_FallbackStaysScopedToTheGuardsWindow(t *testing.T) {
	runAcceptanceTest(t,
		"fleet mode + a LIVE provisioned worktree passes through verbatim (no repo-less scratch dir for normal retros)",
		"TestRetroWorktree_FleetProvisionedWorktreePassesThroughVerbatim", retroPkg)
	runAcceptanceTest(t,
		"non-fleet dispatch never rewrites the operator's designated worktree",
		"TestRetroWorktree_NonFleetStalePathPassesThroughVerbatim", retroPkg)
	runAcceptanceTest(t,
		"the already-fixed EMPTY shape (cycle-1270) stays green — same contract, not a separate fix",
		"TestRetroWorktree_FleetScratchCwdSatisfiesBridgeGuardPredicate", retroPkg)
}

// TestC1278_003_TeardownClearsActiveWorktree is AC2, the root-cause companion,
// driven through the real production seam (newCycleRun's cleanup closure — the
// one RunCycle defers) and asserted on the PERSISTED cycle state, which is what a
// later dispatch actually reads. The second entry is the negative case: a
// preserved worktree must keep its path or `--resume`/`cycle reset` orphan the
// lane's audited work.
func TestC1278_003_TeardownClearsActiveWorktree(t *testing.T) {
	runAcceptanceTest(t,
		"cs.ActiveWorktree is cleared in the persisted cycle state once the lane teardown prune succeeds",
		"TestCycleRunTeardown_ClearsActiveWorktreeAfterPrune", corePkg)
	runAcceptanceTest(t,
		"a PRESERVED worktree (ship-stage failure or abnormal exit) keeps its path — resume reclaims the lane by it",
		"TestCycleRunTeardown_PreservedWorktreeKeepsActiveWorktree", corePkg)
}

// TestC1278_004_ScratchCwdItselfSatisfiesTheGuard is the edge axis and the other
// half of the join, with no subprocess: retro's fallback is only as good as the
// value it falls back TO. Invoking the public helper directly and asserting its
// return against the guard's own predicate pins that the fix cannot be defeated
// by a fallback that mints a path the bridge would reject anyway.
func TestC1278_004_ScratchCwdItselfSatisfiesTheGuard(t *testing.T) {
	ws := t.TempDir()

	got := gobridge.ScratchCwd(ws, "retro-scratch-cwd")
	if got == "" {
		t.Fatalf("ScratchCwd(%q) minted nothing for an owned workspace — retro's fallback then resolves to \"\" and the fleet bridge refuses the launch with errWorktreeRequired", ws)
	}
	fi, err := os.Stat(got)
	if err != nil || !fi.IsDir() {
		t.Fatalf("ScratchCwd returned %q, which is not an existing directory (%v) — it fails the bridge guard's isDir() check, so falling back to it fixes nothing", got, err)
	}
	if !strings.HasPrefix(got, ws+string(filepath.Separator)) {
		t.Errorf("ScratchCwd minted %q outside the workspace %q it was given — the disposable cwd must live where the lane already has write authority", got, ws)
	}

	// Degenerate input: no workspace ⇒ nothing to mint ⇒ "" (never a fabricated
	// path). This is the shape TestRetroWorktree_FleetNeverEmitsANonExistentPath
	// relies on downstream.
	if empty := gobridge.ScratchCwd("", "retro-scratch-cwd"); empty != "" {
		t.Errorf("ScratchCwd(\"\") fabricated %q — with no owned workspace there is nowhere safe to mint and the only honest answer is the empty string", empty)
	}
}

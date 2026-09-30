//go:build acs

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

func runAcceptanceTest(t *testing.T, criterion, name, pkg string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		cmd := exec.Command("go", "test", "-count=1", "-v", "-run", "^"+name+"$", pkg)
		cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
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

func TestC1278_001_StaleWorktreeFallsBackToScratchCwd(t *testing.T) {
	runAcceptanceTest(t,
		"fleet mode + a torn-down lane's stale worktree resolves to an existing scratch dir under the workspace retro owns",
		"TestRetroWorktree_StaleNonExistentPathFallsBackToScratchCwd", retroPkg)
	runAcceptanceTest(t,
		"retro never emits a non-empty path that fails the bridge guard's isDir() check, even with no workspace to mint under",
		"TestRetroWorktree_FleetNeverEmitsANonExistentPath", retroPkg)
}

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

func TestC1278_003_TeardownClearsActiveWorktree(t *testing.T) {
	runAcceptanceTest(t,
		"cs.ActiveWorktree is cleared in the persisted cycle state once the lane teardown prune succeeds",
		"TestCycleRunTeardown_ClearsActiveWorktreeAfterPrune", corePkg)
	runAcceptanceTest(t,
		"a PRESERVED worktree (ship-stage failure or abnormal exit) keeps its path — resume reclaims the lane by it",
		"TestCycleRunTeardown_PreservedWorktreeKeepsActiveWorktree", corePkg)
}

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

	if empty := gobridge.ScratchCwd("", "retro-scratch-cwd"); empty != "" {
		t.Errorf("ScratchCwd(\"\") fabricated %q — with no owned workspace there is nowhere safe to mint and the only honest answer is the empty string", empty)
	}
}

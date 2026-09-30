//go:build acs

package cycle1356

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const mainPkg = "./cmd/evolve"

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runNamed(t *testing.T, names ...string) (ok bool, missing []string, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go",
		"test", "-C", goDir(t), "-count=1", "-v",
		"-run", "^("+strings.Join(names, "|")+")$", mainPkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to LAUNCH for %s: code=%d err=%v\n%s", mainPkg, code, err, tail(out, 30))
	}
	for _, n := range names {
		if !strings.Contains(out, "--- PASS: "+n) {
			missing = append(missing, n)
		}
	}
	return code == 0 && len(missing) == 0, missing, out
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func TestC1356_001_boundary_refresh_fires_only_at_boundary_never_midbatch(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestRunLoopChain_BoundaryRefreshCheckedBeforeEveryBatchNeverMidBatch",
		"TestRunLoopChain_BoundaryRefreshStopsChainBeforeThatBoundarysBatch",
		"TestMaybeRefreshChainBoundary_LagTriggersRebuildRepinReExecAndLedger",
		"TestRunLoop_BoundaryRefreshNeverCalledInsideDispatchHelpers")
	if !ok {
		t.Errorf("chain-boundary self-heal / never-mid-batch wiring regressed (missing PASS receipts: %v). This is Task 1's functional core (chain-boundary-binary-refresh).\n%s", missing, tail(out, 25))
	}
}

func TestC1356_002_staleness_check_failure_fails_open(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestMaybeRefreshChainBoundary_AheadCheckErrorDegradesToNoRefresh",
		"TestDefaultChainBoundaryAhead_GitFailureDegradesToSkip",
		"TestMaybeRefreshChainBoundary_RebuildFailureDegradesToNoRefresh")
	if !ok {
		t.Errorf("boundary-refresh fail-open contract regressed (missing PASS receipts: %v). AC3 requires every staleness/rebuild/repin failure to degrade to refreshed=false, never a halt.\n%s", missing, tail(out, 25))
	}
}

func TestC1356_003_authorization_class_is_distinguishable_from_boot(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestMaybeRefreshChainBoundary_LagTriggersRebuildRepinReExecAndLedger",
		"TestLastChainBoundaryRefreshLogEntry_ReturnsMostRecentEntry",
		"TestChainResultAndLoopResult_BoundaryRefreshJSONTagPresent")
	if !ok {
		t.Errorf("boundary-refresh auditable authorization-class wiring regressed (missing PASS receipts: %v). AC4 requires the boundary class to stay distinguishable from the boot-time class in the audit trail.\n%s", missing, tail(out, 25))
	}
}

func TestC1356_004_parent_branch_heals_child_branch_is_documented_noop(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestMaybeRefreshChainBoundary_PrePinsBeforeReExecSoChildBootRepinIsNoOp",
		"TestAttemptBootRepin_NoOpWhenPinAlreadyMatchesOnDiskBinary")
	if !ok {
		t.Errorf("parent-heals/child-no-ops repin-branch split regressed (missing PASS receipts: %v). This is Task 2 (pin-boundary-repin-branch-residual), reframed onto the boundary-refresh mechanism this worktree actually carries.\n%s", missing, tail(out, 25))
	}
}

func TestC1356_005_no_superseded_stop_only_design_reintroduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	chainSrc := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_chain.go")
	for _, needle := range []string{"chain_binary_stale", "StaleAtBoundary", "StaleBinaryCommit"} {
		if !acsassert.FileNotContains(t, chainSrc, needle) {
			t.Errorf("found %q in cmd_loop_chain.go — a second, superseded staleness-stop code path has been (re-)introduced alongside the shipped chain_boundary_refresh_reexec design; centralize on the one shipped path instead of duplicating it", needle)
		}
	}
}

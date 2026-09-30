//go:build acs

package cycle1370

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

func TestC1370_001_wave_boundary_call_site_fires_before_dispatch_never_midlane(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestRunLoop_CallsMaybeRefreshChainBoundaryAtWaveBoundary",
		"TestRunLoop_BoundaryRefreshNeverCalledInsideDispatchHelpers")
	if !ok {
		t.Errorf("wave/fleet-boundary refresh wiring regressed (missing PASS receipts: %v). This call site is the literal AC1 ask — a regression here reopens the gap the inbox item exists to close.\n%s", missing, tail(out, 25))
	}
}

func TestC1370_002_chain_boundary_call_site_fires_before_every_batch_never_midbatch(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestRunLoopChain_BoundaryRefreshCheckedBeforeEveryBatchNeverMidBatch",
		"TestRunLoopChain_BoundaryRefreshStopsChainBeforeThatBoundarysBatch",
		"TestMaybeRefreshChainBoundary_LagTriggersRebuildRepinReExecAndLedger")
	if !ok {
		t.Errorf("chain-boundary self-heal / never-mid-batch wiring regressed (missing PASS receipts: %v).\n%s", missing, tail(out, 25))
	}
}

func TestC1370_003_failopen_and_fleetlane_guard_refuse_rebuild(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestMaybeRefreshChainBoundary_AheadCheckErrorDegradesToNoRefresh",
		"TestDefaultChainBoundaryAhead_GitFailureDegradesToSkip",
		"TestMaybeRefreshChainBoundary_RebuildFailureDegradesToNoRefresh",
		"TestMaybeRefreshChainBoundary_FleetLaneActiveRefusesRebuild",
		"TestMaybeRefreshChainBoundary_FleetLaneCheckErrorRefusesRebuild")
	if !ok {
		t.Errorf("boundary-refresh fail-open / fleet-lane-guard contract regressed (missing PASS receipts: %v). AC3/AC4 require every staleness/rebuild/fleet-lane check that cannot prove the plane is idle to refuse the rebuild, never halt or clobber a sibling lane.\n%s", missing, tail(out, 25))
	}
}

func TestC1370_004_loop_breaker_refuses_second_attempt_and_ledgers_class(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestMaybeRefreshChainBoundary_SecondAttemptSameCommitIsRefusedLoopBreaker",
		"TestLastChainBoundaryRefreshLogEntry_ReturnsMostRecentEntry",
		"TestChainResultAndLoopResult_BoundaryRefreshJSONTagPresent")
	if !ok {
		t.Errorf("boundary-refresh loop-breaker / auditable authorization-class wiring regressed (missing PASS receipts: %v). AC5 requires a second same-commit attempt to be refused without a second rebuild; AC2 requires the boundary class to stay distinguishable in the audit trail.\n%s", missing, tail(out, 25))
	}
}

func TestC1370_005_no_superseded_stop_only_design_reintroduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	chainSrc := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_chain.go")
	for _, needle := range []string{"chain_binary_stale", "StaleAtBoundary", "StaleBinaryCommit"} {
		if !acsassert.FileNotContains(t, chainSrc, needle) {
			t.Errorf("found %q in cmd_loop_chain.go — a second, superseded staleness-stop code path has been (re-)introduced alongside the shipped chain_boundary_refresh_reexec design; centralize on the one shipped path instead of duplicating it", needle)
		}
	}
}

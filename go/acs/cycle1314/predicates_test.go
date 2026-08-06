//go:build acs

// Package cycle1314 materialises the cycle-1314 acceptance criteria for the
// single fleet-scoped task pinned to this lane: boundary-binary-refresh
// (inbox item auto-refresh-binary-at-boundary, P1, weight 0.94).
//
// The defect: runLoopChain (go/cmd/evolve/cmd_loop_chain.go) relaunches
// runLoopBatchFn at every boundary but never checks whether the running
// binary has fallen behind HEAD — fixes that land on main mid-chain sit inert
// until an operator manually rebuilds + re-pins + relaunches (cost: cycles
// 1302-1309 kept running an already-fixed defect).
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…1098
// precedent). The subject lives in `package main` (go/cmd/evolve), which
// cannot be imported, so each predicate shells `go test -run` over the RED
// contract tests authored this cycle in
// go/cmd/evolve/cmd_loop_chain_boundaryrefresh_test.go. Every one of those
// drives real system-under-test behaviour — a real git fixture for the
// ahead-of-HEAD detection, and an end-to-end runLoopChain drive over spied
// rebuild/repin/re-exec seams — asserting on returned values, the on-disk
// state.json pin, the boundary-refresh audit log, and the emitted chain
// summary. None is a source-grep of production code (the cycle-85
// degenerate-predicate ban). RED now: chainBoundaryAheadFn / chainRebuildFn /
// chainReExecFn / maybeRefreshChainBoundary are all undefined, so
// go/cmd/evolve does not compile.
package cycle1314

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// loopPkg is the chain loop's home package (package main — importable only
// via `go test`, hence the subprocess form).
const loopPkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the
// test cache so the predicate always exercises current source. A compile
// failure in the target package (the expected RED signal before Builder
// implements the seams) surfaces as a non-zero exit.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	// code < 0 is a genuine launch failure (binary missing / killed by signal),
	// not a test verdict; SubprocessOutput returns non-nil err for ANY
	// non-zero exit, so a plain compile/assertion failure (the RED signal)
	// must flow through as ok=false, NOT be misread as "failed to launch".
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC1314_001_AheadCheckDetectsStaleBinaryReusingAncestorIdiom — AC1: a new
// helper detects "running binary behind HEAD" using the same ancestor-check
// idiom runResetSHA already uses (no duplicated logic), including the
// no-lag, empty-commit, and git-failure edge cases.
func TestC1314_001_AheadCheckDetectsStaleBinaryReusingAncestorIdiom(t *testing.T) {
	ok, out := runGoTest(t, loopPkg,
		"TestDefaultChainBoundaryAhead_DetectsRunningCommitBehindHead|TestDefaultChainBoundaryAhead_NoLagWhenRunningCommitIsHead|TestDefaultChainBoundaryAhead_EmptyRunningCommitIsNoOp|TestDefaultChainBoundaryAhead_GitFailureDegradesToSkip")
	if !ok {
		t.Errorf("the ahead-of-HEAD staleness detector is missing or does not correctly reuse the "+
			"ancestor-check idiom (positive/no-lag/empty/git-failure cases):\n%s", out)
	}
}

// TestC1314_002_NoLagSequenceIsFree — AC6c: when no lag is detected, the
// rebuild/repin/re-exec sequence must never fire — the no-op path is provably
// free, not merely "happens not to rebuild this time".
func TestC1314_002_NoLagSequenceIsFree(t *testing.T) {
	ok, out := runGoTest(t, loopPkg, "TestMaybeRefreshChainBoundary_NoLagIsNoOpFree")
	if !ok {
		t.Errorf("a no-lag boundary must call NEITHER rebuild NOR re-exec:\n%s", out)
	}
}

// TestC1314_003_LagTriggersRebuildRepinReExecAndAuditableLedger — AC3 + AC6b:
// on detected lag, the sequence rebuilds, re-pins the ship SHA, writes a
// DISTINGUISHABLE "boundary-refresh" audit record (auditable, not silent —
// the inbox item's explicit requirement), and re-execs with a non-empty argv,
// in rebuild-then-reexec order.
func TestC1314_003_LagTriggersRebuildRepinReExecAndAuditableLedger(t *testing.T) {
	ok, out := runGoTest(t, loopPkg, "TestMaybeRefreshChainBoundary_LagTriggersRebuildRepinReExecAndLedger")
	if !ok {
		t.Errorf("detected lag did not rebuild+repin+re-exec with a distinguishable, ledgered "+
			"boundary-refresh authorization class:\n%s", out)
	}
}

// TestC1314_004_FailuresAtEveryStageDegradeWithoutHalting — AC4: a rebuild
// failure or an ahead-check (git/network) failure must degrade to
// refreshed=false, be logged (not swallowed), and the pin must stay
// untouched on a rebuild failure — the chain keeps running the CURRENT
// binary rather than halting.
func TestC1314_004_FailuresAtEveryStageDegradeWithoutHalting(t *testing.T) {
	ok, out := runGoTest(t, loopPkg,
		"TestMaybeRefreshChainBoundary_RebuildFailureDegradesToNoRefresh|TestMaybeRefreshChainBoundary_AheadCheckErrorDegradesToNoRefresh")
	if !ok {
		t.Errorf("a rebuild or ahead-check failure does not cleanly degrade to refreshed=false "+
			"(or the chain would halt / leave the pin dirty):\n%s", out)
	}
}

// TestC1314_005_BoundaryRefreshNeverInterruptsAnInFlightBatch — AC2: the
// refresh check fires exactly once per boundary, strictly BEFORE that
// boundary's runLoopBatchFn call, over a multi-batch chain — proving it can
// never interleave with an in-flight batch.
func TestC1314_005_BoundaryRefreshNeverInterruptsAnInFlightBatch(t *testing.T) {
	ok, out := runGoTest(t, loopPkg, "TestRunLoopChain_BoundaryRefreshCheckedBeforeEveryBatchNeverMidBatch")
	if !ok {
		t.Errorf("the boundary-refresh check is not strictly ordered before every boundary's batch "+
			"(risk of interrupting an in-flight batch):\n%s", out)
	}
}

// TestC1314_006_TrippedBoundaryStopsChainBeforeItsOwnBatch — AC6a/AC6b
// end-to-end: when the ahead-check trips at a boundary, that boundary runs
// ZERO batches of its own (re-exec is terminal in production) and the chain
// summary names the boundary-refresh stop reason.
func TestC1314_006_TrippedBoundaryStopsChainBeforeItsOwnBatch(t *testing.T) {
	ok, out := runGoTest(t, loopPkg, "TestRunLoopChain_BoundaryRefreshStopsChainBeforeThatBoundarysBatch")
	if !ok {
		t.Errorf("a tripped boundary must run zero of its own batches and name the boundary-refresh "+
			"stop reason in the chain summary:\n%s", out)
	}
}

// TestC1314_007_ExistingChainSemanticsUnchanged — AC5: anti-regression. The
// cycle-1075/1098 chain contract (drain->next batch, quota defer, exact cap,
// brake, fleet-width preservation, rc mapping, min-one-batch, inbox
// pending-validity) must still hold after wiring in the boundary refresh.
// This is the predicate that fails if Builder "wires in" the refresh by
// weakening the pre-existing decision precedence.
func TestC1314_007_ExistingChainSemanticsUnchanged(t *testing.T) {
	ok, out := runGoTest(t, loopPkg, "TestRunLoopChain_.*|TestChain.*Decision.*|TestInboxPendingCount.*|TestParseLoopArgs_UntilInboxEmpty")
	if !ok {
		t.Errorf("the pre-existing chain contract regressed while wiring in boundary-binary-refresh:\n%s", out)
	}
}

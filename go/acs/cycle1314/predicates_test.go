//go:build acs

package cycle1314

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const loopPkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC1314_001_AheadCheckDetectsStaleBinaryReusingAncestorIdiom(t *testing.T) {
	ok, out := runGoTest(t, loopPkg,
		"TestDefaultChainBoundaryAhead_DetectsRunningCommitBehindHead|TestDefaultChainBoundaryAhead_NoLagWhenRunningCommitIsHead|TestDefaultChainBoundaryAhead_EmptyRunningCommitIsNoOp|TestDefaultChainBoundaryAhead_GitFailureDegradesToSkip")
	if !ok {
		t.Errorf("the ahead-of-HEAD staleness detector is missing or does not correctly reuse the "+
			"ancestor-check idiom (positive/no-lag/empty/git-failure cases):\n%s", out)
	}
}

func TestC1314_002_NoLagSequenceIsFree(t *testing.T) {
	ok, out := runGoTest(t, loopPkg, "TestMaybeRefreshChainBoundary_NoLagIsNoOpFree")
	if !ok {
		t.Errorf("a no-lag boundary must call NEITHER rebuild NOR re-exec:\n%s", out)
	}
}

func TestC1314_003_LagTriggersRebuildRepinReExecAndAuditableLedger(t *testing.T) {
	ok, out := runGoTest(t, loopPkg, "TestMaybeRefreshChainBoundary_LagTriggersRebuildRepinReExecAndLedger")
	if !ok {
		t.Errorf("detected lag did not rebuild+repin+re-exec with a distinguishable, ledgered "+
			"boundary-refresh authorization class:\n%s", out)
	}
}

func TestC1314_004_FailuresAtEveryStageDegradeWithoutHalting(t *testing.T) {
	ok, out := runGoTest(t, loopPkg,
		"TestMaybeRefreshChainBoundary_RebuildFailureDegradesToNoRefresh|TestMaybeRefreshChainBoundary_AheadCheckErrorDegradesToNoRefresh")
	if !ok {
		t.Errorf("a rebuild or ahead-check failure does not cleanly degrade to refreshed=false "+
			"(or the chain would halt / leave the pin dirty):\n%s", out)
	}
}

func TestC1314_005_BoundaryRefreshNeverInterruptsAnInFlightBatch(t *testing.T) {
	ok, out := runGoTest(t, loopPkg, "TestRunLoopChain_BoundaryRefreshCheckedBeforeEveryBatchNeverMidBatch")
	if !ok {
		t.Errorf("the boundary-refresh check is not strictly ordered before every boundary's batch "+
			"(risk of interrupting an in-flight batch):\n%s", out)
	}
}

func TestC1314_006_TrippedBoundaryStopsChainBeforeItsOwnBatch(t *testing.T) {
	ok, out := runGoTest(t, loopPkg, "TestRunLoopChain_BoundaryRefreshStopsChainBeforeThatBoundarysBatch")
	if !ok {
		t.Errorf("a tripped boundary must run zero of its own batches and name the boundary-refresh "+
			"stop reason in the chain summary:\n%s", out)
	}
}

func TestC1314_007_ExistingChainSemanticsUnchanged(t *testing.T) {
	ok, out := runGoTest(t, loopPkg, "TestRunLoopChain_.*|TestChain.*Decision.*|TestInboxPendingCount.*|TestParseLoopArgs_UntilInboxEmpty")
	if !ok {
		t.Errorf("the pre-existing chain contract regressed while wiring in boundary-binary-refresh:\n%s", out)
	}
}

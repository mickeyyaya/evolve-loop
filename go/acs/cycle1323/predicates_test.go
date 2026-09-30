//go:build acs

package cycle1323

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const chainPkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

func runChainTests(t *testing.T, runExpr string, wantPass []string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", runExpr, "-v", chainPkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %q %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			runExpr, chainPkg, code, err, stdout, stderr)
	}
	for _, name := range wantPass {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("test %s did not report PASS (renamed, skipped, or not authored)\nstdout:\n%s", name, stdout)
		}
	}
}

func TestC1323_001_boundary_refresh_provenance_gate_is_real(t *testing.T) {
	runChainTests(t,
		"^(TestMaybeRefreshChainBoundary_UnverifiedProvenanceRefusesRepinAndReExec|TestDefaultChainBoundaryRepinProvenance_RejectsNonAncestorAndEmptyCommits)$",
		[]string{
			"TestMaybeRefreshChainBoundary_UnverifiedProvenanceRefusesRepinAndReExec",
			"TestDefaultChainBoundaryRepinProvenance_RejectsNonAncestorAndEmptyCommits",
		})
}

func TestC1323_002_repin_hashes_the_rebuilt_binary(t *testing.T) {
	runChainTests(t,
		"^TestMaybeRefreshChainBoundary_PinsShaOfRebuiltBinaryNotRunningExecutable$",
		[]string{"TestMaybeRefreshChainBoundary_PinsShaOfRebuiltBinaryNotRunningExecutable"})
}

func TestC1323_003_reexec_targets_the_rebuilt_binary(t *testing.T) {
	runChainTests(t,
		"^TestMaybeRefreshChainBoundary_(ReExecTargetsRebuiltBinaryNotArgv0|MissingRebuiltBinaryDegradesToNoRefresh)$",
		[]string{
			"TestMaybeRefreshChainBoundary_ReExecTargetsRebuiltBinaryNotArgv0",
			"TestMaybeRefreshChainBoundary_MissingRebuiltBinaryDegradesToNoRefresh",
		})
}

func TestC1323_004_reexec_loop_breaker_bounds_the_chain(t *testing.T) {
	runChainTests(t,
		"^(TestMaybeRefreshChainBoundary_SecondAttemptSameCommitIsRefusedLoopBreaker|TestMaybeRefreshChainBoundary_LoopBreakerRearmsWhenCommitMoves|TestRunLoopChain_LoopBreakerLetsBatchesRunAfterAFruitlessReExec)$",
		[]string{
			"TestMaybeRefreshChainBoundary_SecondAttemptSameCommitIsRefusedLoopBreaker",
			"TestMaybeRefreshChainBoundary_LoopBreakerRearmsWhenCommitMoves",
			"TestRunLoopChain_LoopBreakerLetsBatchesRunAfterAFruitlessReExec",
		})
}

func TestC1323_005_sentinel_removed_and_prior_contract_intact(t *testing.T) {
	runChainTests(t,
		"^TestMaybeRefreshChainBoundary_NeverSubstitutesSentinelForEmptyCommit$",
		[]string{"TestMaybeRefreshChainBoundary_NeverSubstitutesSentinelForEmptyCommit"})

	runChainTests(t,
		"^(TestDefaultChainBoundaryAhead_|TestMaybeRefreshChainBoundary_NoLagIsNoOpFree|TestMaybeRefreshChainBoundary_LagTriggersRebuildRepinReExecAndLedger|TestMaybeRefreshChainBoundary_RebuildFailureDegradesToNoRefresh|TestRunLoopChain_BoundaryRefresh)",
		[]string{
			"TestDefaultChainBoundaryAhead_DetectsRunningCommitBehindHead",
			"TestDefaultChainBoundaryAhead_NoLagWhenRunningCommitIsHead",
			"TestDefaultChainBoundaryAhead_EmptyRunningCommitIsNoOp",
			"TestDefaultChainBoundaryAhead_GitFailureDegradesToSkip",
			"TestDefaultChainBoundaryAhead_ShortCommitAtHeadIsNotAhead",
			"TestDefaultChainBoundaryAhead_ShortCommitBehindHeadIsAhead",
			"TestMaybeRefreshChainBoundary_NoLagIsNoOpFree",
			"TestMaybeRefreshChainBoundary_LagTriggersRebuildRepinReExecAndLedger",
			"TestMaybeRefreshChainBoundary_RebuildFailureDegradesToNoRefresh",
			"TestRunLoopChain_BoundaryRefreshCheckedBeforeEveryBatchNeverMidBatch",
			"TestRunLoopChain_BoundaryRefreshStopsChainBeforeThatBoundarysBatch",
		})
}

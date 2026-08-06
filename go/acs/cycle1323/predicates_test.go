//go:build acs

// Package cycle1323 materialises the cycle-1323 acceptance criteria for the one
// fleet-scoped task pinned to this lane: `auto-refresh-binary-at-boundary`.
//
// Cycle 1323 is a CONTINUATION of cycle 1320. The boundary-refresh sequence
// (ahead-check -> rebuild -> repin -> ledger -> re-exec) already exists in
// go/cmd/evolve/cmd_loop_chain.go; cycle 1320's audit FAILed it with 9 defects,
// of which 2 (D1 short-sha ahead-check, D2 short-sha test gap) are already fixed
// in this tree. This cycle's acceptance bar is the remaining OPEN set:
//
//	AC1  dd8a8d64 / dcaf44e4 — the repin's ProvenanceVerified control is a real
//	     git-ancestor check, not the `func(string) bool { return true }` stub
//	     that stamps Authorized="provenance" on an unverified pin (ADR-0072
//	     forged-verdict class).
//	AC2  d7542cf6 — the repin pins sha256(<root>/go/bin/evolve), the binary the
//	     rebuild just produced, not selfsha.Running() (the stale running image).
//	AC3  ddb8f717 — the re-exec targets that same rebuilt binary, not
//	     exec.LookPath(os.Args[0]).
//	AC4  de8b9e49 / df20cf48 — an on-disk loop breaker caps re-execs for one
//	     unchanged build commit, so an ahead-check false positive degrades to
//	     "run batches on the current binary" instead of bricking the chain at
//	     zero batches executed.
//	AC5  d9d245d4 — the dead `repinCommit = "boundary-refresh"` sentinel is gone
//	     and can never be laundered past the provenance gate.
//
// Predicate strategy. Every predicate below EXERCISES the production function
// (`maybeRefreshChainBoundary`) or its production caller (`runLoopChain`) by
// running the named RED tests in go/cmd/evolve — never a source-grep of
// production text (the cycle-85 degenerate-predicate ban): a predicate that only
// asserted "cmd_loop_chain.go no longer contains `return true`" would pass on a
// cosmetic rename. Each run is narrowed with `-run` to exactly the tests that
// drive the changed seam (the flaky-predicate rule against whole-package
// ./cmd/evolve sweeps, cycles 1173/1175/1178) and demands an explicit
// `--- PASS: <name>` line per test, so a rename or a skip can never satisfy a
// predicate on exit code alone.
package cycle1323

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// chainPkg is the package under test — addressed by full import path so the
// predicate resolves regardless of the cwd `go test` hands this package.
const chainPkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

// runChainTests runs the named cmd/evolve tests (fresh, verbose) and requires an
// explicit "--- PASS: <name>" for every wantPass. Exit 0 alone never satisfies a
// predicate: a renamed, skipped, or never-authored test emits no PASS line.
// Always narrowed by runExpr — never a whole ./cmd/evolve sweep.
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

// -----------------------------------------------------------------------------
// AC1 — the provenance gate is real (anti-forgery). NEGATIVE-first: the
// load-bearing assertion is that an UNVERIFIED build commit is REFUSED, which no
// always-true stub can satisfy.
// -----------------------------------------------------------------------------

func TestC1323_001_boundary_refresh_provenance_gate_is_real(t *testing.T) {
	runChainTests(t,
		"^(TestMaybeRefreshChainBoundary_UnverifiedProvenanceRefusesRepinAndReExec|TestDefaultChainBoundaryRepinProvenance_RejectsNonAncestorAndEmptyCommits)$",
		[]string{
			// Negative: unverified commit ⇒ pin untouched, no ledger, no re-exec.
			"TestMaybeRefreshChainBoundary_UnverifiedProvenanceRefusesRepinAndReExec",
			// The SHIPPED default closure rejects a non-ancestor and an empty
			// commit and accepts a real ancestor — proof the seam is not a
			// constant in production.
			"TestDefaultChainBoundaryRepinProvenance_RejectsNonAncestorAndEmptyCommits",
		})
}

// -----------------------------------------------------------------------------
// AC2 — the pin is sha256 of the REBUILT binary, not the running image.
// -----------------------------------------------------------------------------

func TestC1323_002_repin_hashes_the_rebuilt_binary(t *testing.T) {
	runChainTests(t,
		"^TestMaybeRefreshChainBoundary_PinsShaOfRebuiltBinaryNotRunningExecutable$",
		[]string{"TestMaybeRefreshChainBoundary_PinsShaOfRebuiltBinaryNotRunningExecutable"})
}

// -----------------------------------------------------------------------------
// AC3 — the re-exec lands on the rebuilt binary; an absent one degrades to no
// refresh (edge case) rather than exec'ing an unknown path.
// -----------------------------------------------------------------------------

func TestC1323_003_reexec_targets_the_rebuilt_binary(t *testing.T) {
	runChainTests(t,
		"^TestMaybeRefreshChainBoundary_(ReExecTargetsRebuiltBinaryNotArgv0|MissingRebuiltBinaryDegradesToNoRefresh)$",
		[]string{
			"TestMaybeRefreshChainBoundary_ReExecTargetsRebuiltBinaryNotArgv0",
			// Edge/negative: nothing safe to exec into ⇒ no refresh, pin intact.
			"TestMaybeRefreshChainBoundary_MissingRebuiltBinaryDegradesToNoRefresh",
		})
}

// -----------------------------------------------------------------------------
// AC4 — the re-exec loop breaker, including the REACHABILITY proof through the
// production caller runLoopChain (a seam whose only driver is a helper-level
// unit test proves nothing about the bricked-chain defect).
// -----------------------------------------------------------------------------

func TestC1323_004_reexec_loop_breaker_bounds_the_chain(t *testing.T) {
	runChainTests(t,
		"^(TestMaybeRefreshChainBoundary_SecondAttemptSameCommitIsRefusedLoopBreaker|TestMaybeRefreshChainBoundary_LoopBreakerRearmsWhenCommitMoves|TestRunLoopChain_LoopBreakerLetsBatchesRunAfterAFruitlessReExec)$",
		[]string{
			// Negative: the second attempt on an unchanged commit is refused.
			"TestMaybeRefreshChainBoundary_SecondAttemptSameCommitIsRefusedLoopBreaker",
			// The breaker is per-commit, not a permanent kill switch.
			"TestMaybeRefreshChainBoundary_LoopBreakerRearmsWhenCommitMoves",
			// Reachability: driven from runLoopChain, batches actually run.
			"TestRunLoopChain_LoopBreakerLetsBatchesRunAfterAFruitlessReExec",
		})
}

// -----------------------------------------------------------------------------
// AC5 — the dead "boundary-refresh" sentinel commit is gone, plus a no-regression
// guard over cycle-1320's surviving boundary-refresh contract (the ahead-check,
// the ordering, the ledger, and the fail-open degrades must all still hold after
// this cycle's rework of the repin/re-exec path).
// -----------------------------------------------------------------------------

func TestC1323_005_sentinel_removed_and_prior_contract_intact(t *testing.T) {
	runChainTests(t,
		"^TestMaybeRefreshChainBoundary_NeverSubstitutesSentinelForEmptyCommit$",
		[]string{"TestMaybeRefreshChainBoundary_NeverSubstitutesSentinelForEmptyCommit"})

	// No-regression: every boundary-refresh test cycle 1320 left GREEN must
	// still be GREEN after the repin/re-exec rework.
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

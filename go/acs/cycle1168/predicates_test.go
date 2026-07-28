//go:build acs

// Package cycle1168 holds the cycle-1168 ACS predicates.
//
// Cycle-1168 committed ONE task (triage `## top_n`):
//
//  1. close-out-evaluate-batch-retry-parity-tracker (fleet_scope: evaluate-batch-retry-parity)
//
// The deferred item (reevaluate-paralleleval-stageoff-soak) gets ZERO predicates
// per R9.3: predicates bind only to triage-committed work.
//
// The task is a tracker correction: the code-audit README still lists
// `evaluate-batch-retry-parity` as an OPEN blocker ("missing optionalInfraSkip
// — blocks the enforce flip") although the parity hooks landed in cycle-1166
// (retry_opts.go: evaluateBatchRetryOpts wires both degrade predicates and
// dispatchRunnerWithRetry delegates to the shared retry core). A stale row keeps
// re-surfacing resolved work into future fleet scopes.
//
// Predicate shape. The doc row is the deliverable, so its two predicates assert
// the artifact directly (positive row state + the anti-no-op negative that RED-
// fails on the untouched repo). The two claim-substantiation predicates EXERCISE
// the system under test by running the cycle's parity unit tests against the
// worktree build — the tracker may only be marked resolved if the behavior it
// tracked is actually green, so "annotate the row" alone cannot green this suite.
package cycle1168

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// trackerPath is the code-audit tracker whose row this cycle corrects.
const trackerPath = "knowledge-base/research/code-audit-2026-07/README.md"

// itemID is the audit item (and this cycle's fleet_scope todo-id).
const itemID = "evaluate-batch-retry-parity"

// tracker returns the tracker file's absolute path under the cycle worktree.
func tracker(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), trackerPath)
}

// goTest runs `go test` inside the cycle worktree's go module and returns
// combined output plus the exit code. `-C` is used (rather than a cwd change)
// because acsassert.SubprocessOutput runs in the predicate's own working
// directory, which is not the module root.
func goTest(t *testing.T, pkg string, runRegex string) (string, int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	args := []string{"-C", filepath.Join(root, "go"), "test", "-count=1"}
	if runRegex != "" {
		args = append(args, "-run", runRegex)
	}
	args = append(args, pkg)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", args...)
	if err != nil && code == -1 {
		t.Fatalf("could not run go test (%s %s): %v", pkg, runRegex, err)
	}
	return stdout + stderr, code
}

// TestC1168_001_TrackerRowRecordsResolution — AC-1, positive half.
//
// The item's table row must read as RESOLVED and must cite where the resolution
// lives, so the next scout that reads this tracker can verify the claim without
// re-deriving it. Any of the accepted evidence tokens satisfies the citation
// (the wiring is in retry_opts.go, reached from evaluate_batch.go, landed by
// cycle-1166 and closed out by cycle-1168) — the predicate pins that a citation
// EXISTS on the row, not one particular spelling of it.
func TestC1168_001_TrackerRowRecordsResolution(t *testing.T) {
	path := tracker(t)
	if !acsassert.FileExists(t, path) {
		t.Fatalf("tracker %s is missing", trackerPath)
	}
	if !acsassert.LineContainsAll(path, itemID, "RESOLVED") {
		t.Errorf("no line of %s carries both %q and RESOLVED — the row still reads as open work,\n"+
			"so this item keeps being re-selected into future fleet scopes.", trackerPath, itemID)
	}
	evidence := []string{"retry_opts.go", "evaluate_batch.go", "cycle-1166", "cycle-1168"}
	cited := false
	for _, tok := range evidence {
		if acsassert.LineContainsAll(path, itemID, "RESOLVED", tok) {
			cited = true
			break
		}
	}
	if !cited {
		t.Errorf("the RESOLVED row for %q cites no evidence; expected one of %v on the same line.\n"+
			"An unsourced 'RESOLVED' is unverifiable and will be re-audited blind.", itemID, evidence)
	}
}

// TestC1168_002_TrackerNoLongerClaimsOpenBlocker — AC-1, NEGATIVE half
// (the anti-no-op signal: this predicate FAILS on the untouched repo, so a
// no-op cycle cannot green the suite).
//
// Two stale claims must be gone: the row's defect description ("missing
// optionalInfraSkip" / "blocks the enforce flip") and the sequencing paragraph's
// open constraint ("<item> gates the parallel-evaluate enforce flip"). Absence
// is asserted with FileNotContains, never an inverted FileContains (the
// cycle-352 broken-predicate incident).
func TestC1168_002_TrackerNoLongerClaimsOpenBlocker(t *testing.T) {
	path := tracker(t)
	stale := []string{
		"missing optionalInfraSkip",
		"blocks the enforce flip",
		itemID + " gates the parallel-evaluate enforce flip",
	}
	for _, s := range stale {
		if !acsassert.FileNotContains(t, path, s) {
			t.Errorf("%s still asserts the stale claim %q — the blocker was cleared in cycle-1166;\n"+
				"leaving the prose open is what re-surfaces resolved work.", trackerPath, s)
		}
	}
}

// TestC1168_003_ResolutionClaimIsTrue_ParitySuiteGreen — substantiates AC-1's
// claim behaviorally, and is AC-3's guard against a re-fix.
//
// The tracker may be marked RESOLVED only if the retry parity it tracked really
// holds. This runs the item's own RED contract (the three dispatch-parity tests,
// including the NON-skippable-error-still-fatal negative) plus the amplify suite
// (wrapped-error match, non-infra never matches, mandatory overrides optional,
// floor phases never skip, pre-ship/ship-itself never skip) and the cycle-1166
// hook-set parity pins. A doc edit alone cannot make these pass; a regression in
// the parity code makes the RESOLVED annotation red-fail here.
func TestC1168_003_ResolutionClaimIsTrue_ParitySuiteGreen(t *testing.T) {
	const suite = "TestDispatchRunnerWithRetry_OptionalInfraSkipParity|" +
		"TestDispatchRunnerWithRetry_PostShipObserverSkipParity|" +
		"TestDispatchRunnerWithRetry_NonSkippableErrorStillFatal|" +
		"TestDispatchRunnerWithRetry_DelegatesToTheSharedRetryCore|" +
		"TestEvaluateBatchRetryOpts_WiresBothSkipsButNotShipRecovery|" +
		"TestRetryOpts_EnumeratesEveryDispatchHook|" +
		"TestOptionalInfraSkip_WrappedArtifactTimeoutError_StillMatches|" +
		"TestOptionalInfraSkip_NonInfraError_NeverMatches|" +
		"TestOptionalInfraSkip_MandatoryOverridesOptionalFlag|" +
		"TestOptionalInfraSkip_OnFloorPhase_NeverMatches|" +
		"TestPostShipObserverSkip_NotYetShipped_NeverMatchesRegardlessOfPhase|" +
		"TestPostShipObserverSkip_ShipItself_NeverMatchesEvenIfShipped|" +
		"TestPostShipObserverSkip_MandatoryOverridesEvenWhenShippedAndControl"
	out, code := goTest(t, "./internal/core/", suite)
	if code != 0 {
		t.Errorf("the retry-parity behavior the tracker is being marked RESOLVED for is NOT green (exit %d).\n"+
			"Annotating the row while the parity is broken would launder a live defect into closed work.\n%s",
			code, tail(out))
	}
}

// TestC1168_004_NoRefixOfParityWiring — AC-3 (doc-only task: no code re-fix).
//
// The parity must remain a SINGLE delegated wiring, exactly as cycle-1166 left
// it: evaluateBatchRetryOpts wires each degrade predicate once, and
// dispatchRunnerWithRetry keeps ZERO inline hook calls (it delegates to the
// shared retry core). Counts are function-scoped AST reads, so they cannot be
// satisfied by adding text elsewhere in the file, and the ==0 arm fails LOUDLY
// on a renamed function rather than passing vacuously. A cycle that "re-fixed"
// the item by hand-rolling a second retry loop trips this.
func TestC1168_004_NoRefixOfParityWiring(t *testing.T) {
	root := acsassert.RepoRoot(t)
	optsFile := filepath.Join(root, "go", "internal", "core", "retry_opts.go")
	batchFile := filepath.Join(root, "go", "internal", "core", "evaluate_batch.go")

	for _, hook := range []string{"optionalInfraSkip", "postShipObserverSkip"} {
		n, err := acsassert.CountInGoFunc(optsFile, "evaluateBatchRetryOpts", hook)
		if err != nil {
			t.Errorf("cannot count %s in evaluateBatchRetryOpts: %v", hook, err)
			continue
		}
		if n != 1 {
			t.Errorf("evaluateBatchRetryOpts wires %s %d times, want exactly 1 — "+
				"the batch path's degrade hook set is the parity contract.", hook, n)
		}
	}

	inline, err := acsassert.CountInGoFunc(batchFile, "dispatchRunnerWithRetry", "optionalInfraSkip", "postShipObserverSkip")
	if err != nil {
		t.Fatalf("cannot inspect dispatchRunnerWithRetry: %v", err)
	}
	if inline != 0 {
		t.Errorf("dispatchRunnerWithRetry calls the degrade predicates inline %d times — "+
			"it must DELEGATE to retryPhaseRunner. A second hand-maintained loop is exactly "+
			"how the next hook silently misses the batch path again (cycle-1166).", inline)
	}
}

// TestC1168_005_TouchedPackagesStayGreen — AC-2 regression floor.
//
// The task edits no code, so the packages that own the tracked behavior must be
// wholly green; any red here means the cycle touched code it contracted not to.
func TestC1168_005_TouchedPackagesStayGreen(t *testing.T) {
	for _, pkg := range []string{"./internal/core/", "./internal/config/"} {
		out, code := goTest(t, pkg, "")
		if code != 0 {
			t.Errorf("%s regressed (exit %d) — cycle-1168 is a documentation-only closure "+
				"and must not move any code.\n%s", pkg, code, tail(out))
		}
	}
}

// tail trims subprocess output to the last ~40 lines so a predicate failure
// reports the actual assertion messages without flooding the audit artifact.
func tail(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	return strings.Join(lines, "\n")
}

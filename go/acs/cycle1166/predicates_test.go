//go:build acs

// Package cycle1166 holds the cycle-1166 ACS predicates.
//
// Cycle-1166 committed three tasks (triage `## top_n`):
//
//  1. evaluate-batch-retry-parity            (inbox weight 0.87)
//  2. infra-teardown-predicate-single-source (inbox weight 0.86)
//  3. spine-failopen-telemetry               (inbox weight 0.85)
//
// The deferred item (tokenopt-session-resume-on-retry) gets ZERO predicates per
// R9.3: predicates bind only to triage-committed work.
//
// Each predicate below EXERCISES the system under test by running the cycle's
// RED unit tests against the worktree build — no source-grep gaming. The unit
// tests themselves carry the negative / anti-no-op assertions (a degenerate
// "always degrade", a widened transient-only predicate, an always-firing
// counter each fail their own negative twin), so a predicate that greens here
// implies the real behavior, not a magic string.
package cycle1166

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goTest runs `go test` inside the cycle worktree's go module and returns
// combined output plus the exit code. `-C` is used (rather than a cwd change)
// because acsassert.SubprocessOutput runs in the predicate's own working
// directory, which is not the module root.
func goTest(t *testing.T, pkg string, runRegex string) (string, int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	args := []string{"-C", root + "/go", "test", "-count=1"}
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

// TestC1166_001_EvaluateBatchRetryParityPinned — task evaluate-batch-retry-parity.
//
// The item's fix is "extract the shared retry core … retryOpts is a small
// Strategy struct carrying the optional hooks", and its AC-2 is the parity pin:
// "a table test enumerating retryOpts hooks asserts the main loop passes the
// full set (a new hook added to cyclerun without registering in retryOpts fails
// compilation or the table)". This predicate runs that pin plus the delegation
// proof (dispatchRunnerWithRetry must route through retryPhaseRunner rather than
// keeping a second hand-maintained loop) and the ship-recovery anti-widen
// negative.
func TestC1166_001_EvaluateBatchRetryParityPinned(t *testing.T) {
	const suite = "TestRetryOpts_EnumeratesEveryDispatchHook|" +
		"TestMainDispatchRetryOpts_PassesTheFullHookSet|" +
		"TestEvaluateBatchRetryOpts_WiresBothSkipsButNotShipRecovery|" +
		"TestDispatchRunnerWithRetry_DelegatesToTheSharedRetryCore"
	out, code := goTest(t, "./internal/core/", suite)
	if code != 0 {
		t.Errorf("retryOpts parity pin is not satisfied (exit %d).\n"+
			"The two dispatch retry loops must share one core with an enumerable hook set, "+
			"or the NEXT hook added to cyclerun silently misses the batch path again.\n%s",
			code, tail(out))
	}
}

// TestC1166_002_InfraTeardownUnionSpelledOnce — task
// infra-teardown-predicate-single-source.
//
// AC-2: "(timeout OR transient) is spelled exactly ONCE via IsInfraTeardownError;
// transient-only via isTransientBridgeError". The suite run here contains the
// uniqueness scan (an AST count over internal/core — non-gameable by ADDING
// text, since it fails on a count > 1), the union-semantics pin, the
// equivalence proof that justifies each replacement, and — most importantly —
// the anti-widen negative the item calls its whole risk: isTransientBridgeError
// must keep returning false for ErrArtifactTimeout.
func TestC1166_002_InfraTeardownUnionSpelledOnce(t *testing.T) {
	const suite = "TestIsInfraTeardownError_UnionSemantics|" +
		"TestIsTransientBridgeError_StaysTransientOnly|" +
		"TestOptionalInfraSkip_InfraGateUnchangedAfterConsolidation|" +
		// (renamed 2026-08-24: gate now proves equivalence to the widened
		// single-source IsOptionalSkippableError — cycle-1551)
		"TestOptionalInfraSkip_GateAgreesWithIsOptionalSkippableError|" +
		"TestInfraTeardownUnion_SpelledExactlyOnce"
	out, code := goTest(t, "./internal/core/", suite)
	if code != 0 {
		t.Errorf("the infra-teardown union predicate is still multiply spelled, or a site was "+
			"incorrectly widened (exit %d).\n%s", code, tail(out))
	}
}

// TestC1166_003_SpineFailOpenRecordedInCore — task spine-failopen-telemetry,
// core half. The fail-open branch must record (phase, missing artifact, reason)
// instead of only writing a stderr WARN, and the spine gate must be able to NAME
// the unsatisfied predecessor. Includes the negative twin: a healthy cycle
// records zero events.
func TestC1166_003_SpineFailOpenRecordedInCore(t *testing.T) {
	const suite = "TestUnsatisfiedSpineAnchor_NamesTheMissingPredecessor|" +
		"TestUnsatisfiedSpineAnchor_AgreesWithSpineSatisfiedUpTo|" +
		"TestRecordSpineFailOpen_CarriesPhaseArtifactAndReason|" +
		"TestRecordSpineFailOpen_UnrecordedCycleHasNoFailOpens"
	out, code := goTest(t, "./internal/core/", suite)
	if code != 0 {
		t.Errorf("spine fail-open events are still uncounted in core (exit %d).\n"+
			"76 silent WARNs in one batch is an epidemic without a dashboard.\n%s", code, tail(out))
	}
}

// TestC1166_004_SpineFailOpenSurfacedInDossierAndRollup — task
// spine-failopen-telemetry, dossier half. Runs the item's two verbatim-named
// RED tests (TestSpineFailOpen_CountedInDossierWithPhaseAndArtifact and
// TestLoopSummary_RollsUpSpineFailOpensPerBatch) plus both negatives: the
// omitempty pin (a healthy cycle emits no key) and the clean-batch silence pin
// (a batch with no fail-opens raises no threshold escalation).
func TestC1166_004_SpineFailOpenSurfacedInDossierAndRollup(t *testing.T) {
	const suite = "TestSpineFailOpen_CountedInDossierWithPhaseAndArtifact|" +
		"TestSpineFailOpen_HealthyCycleOmitsTheField|" +
		"TestLoopSummary_RollsUpSpineFailOpensPerBatch|" +
		"TestRollupSpineFailOpens_CleanBatchIsSilent"
	out, code := goTest(t, "./internal/dossier/", suite)
	if code != 0 {
		t.Errorf("spine fail-opens do not reach the dossier / batch rollup (exit %d).\n"+
			"An in-memory counter no operator surface reads is the status quo this task removes.\n%s",
			code, tail(out))
	}
}

// TestC1166_005_TouchedPackagesStayGreen is the regression floor shared by all
// three tasks: task 1's AC "existing cyclerun dispatch behavior unchanged (core
// suite green)" and task 2's "each replaced site's before/after behavior is
// proven identical by its EXISTING test staying green (this is a
// behavior-preserving refactor — no verdict changes)". Both tasks rewrite hot
// dispatch/retry control flow, so the whole-package suites are the load-bearing
// evidence that nothing else moved.
func TestC1166_005_TouchedPackagesStayGreen(t *testing.T) {
	for _, pkg := range []string{"./internal/core/", "./internal/dossier/", "./internal/cyclestate/"} {
		out, code := goTest(t, pkg, "")
		if code != 0 {
			t.Errorf("%s regressed (exit %d) — the retry-core extraction and the predicate "+
				"consolidation are behavior-preserving by contract.\n%s", pkg, code, tail(out))
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

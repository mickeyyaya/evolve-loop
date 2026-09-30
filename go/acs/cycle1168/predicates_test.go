//go:build acs

package cycle1168

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const trackerPath = "docs/research/code-audit-2026-07/README.md"

const itemID = "evaluate-batch-retry-parity"

func tracker(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), trackerPath)
}

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

func TestC1168_005_TouchedPackagesStayGreen(t *testing.T) {
	for _, pkg := range []string{"./internal/core/", "./internal/config/"} {
		out, code := goTest(t, pkg, "")
		if code != 0 {
			t.Errorf("%s regressed (exit %d) — cycle-1168 is a documentation-only closure "+
				"and must not move any code.\n%s", pkg, code, tail(out))
		}
	}
}

func tail(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	return strings.Join(lines, "\n")
}

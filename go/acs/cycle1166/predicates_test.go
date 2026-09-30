//go:build acs

package cycle1166

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

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

func TestC1166_002_InfraTeardownUnionSpelledOnce(t *testing.T) {
	const suite = "TestIsInfraTeardownError_UnionSemantics|" +
		"TestIsTransientBridgeError_StaysTransientOnly|" +
		"TestOptionalInfraSkip_InfraGateUnchangedAfterConsolidation|" +
		"TestOptionalInfraSkip_GateAgreesWithIsOptionalSkippableError|" +
		"TestInfraTeardownUnion_SpelledExactlyOnce"
	out, code := goTest(t, "./internal/core/", suite)
	if code != 0 {
		t.Errorf("the infra-teardown union predicate is still multiply spelled, or a site was "+
			"incorrectly widened (exit %d).\n%s", code, tail(out))
	}
}

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

func TestC1166_005_TouchedPackagesStayGreen(t *testing.T) {
	for _, pkg := range []string{"./internal/core/", "./internal/dossier/", "./internal/cyclestate/"} {
		out, code := goTest(t, pkg, "")
		if code != 0 {
			t.Errorf("%s regressed (exit %d) — the retry-core extraction and the predicate "+
				"consolidation are behavior-preserving by contract.\n%s", pkg, code, tail(out))
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

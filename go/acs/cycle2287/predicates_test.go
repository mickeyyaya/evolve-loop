//go:build acs

package cycle2287

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	planPath = "docs/plans/gc-closes-the-legacy-gap-2026-11.md"

	gcSweepTests = "SweepSealedCycle|SealedCycleReport"

	cmdSweepTests = "TestDispatchSequentialCycle_|TestRunResumeBatch_AResumedCycle|" +
		"TestRunCycleRun_(ASealedFleetLane|TheGCModeGovernsTheSealedFleetLane|TheLiveLaneOfAnotherCycle)"
)

func sealedGCTests(t *testing.T, pkg string, names ...string) {
	t.Helper()
	parents := make([]string, len(names))
	for i, name := range names {
		parents[i], _, _ = strings.Cut(name, "/")
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: pkg,
		Pattern: "^(" + strings.Join(slices.Compact(parents), "|") + ")$",
		Names:   names,
	})
}

func coverFunctions(t *testing.T, pkg, run string) string {
	t.Helper()
	moduleDir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatalf("module dir: %v", err)
	}
	profile := filepath.Join(t.TempDir(), "sealed.cov")
	out, errOut, code, runErr := acsassert.SubprocessOutput("go", "-C", moduleDir, "test", "-count=1", "-coverprofile="+profile, "-run", run, pkg)
	if runErr != nil || code != 0 {
		t.Fatalf("go test %s exit=%d err=%v\n%s", pkg, code, runErr, acsassert.OutputTail(out+errOut, 40))
	}
	cover, coverErr, code, runErr := acsassert.SubprocessOutput("go", "-C", moduleDir, "tool", "cover", "-func="+profile)
	if runErr != nil || code != 0 {
		t.Fatalf("go tool cover exit=%d err=%v\n%s", code, runErr, acsassert.OutputTail(cover+coverErr, 40))
	}
	return cover
}

func assertFullyCovered(t *testing.T, cover string, functions ...string) {
	t.Helper()
	for _, fn := range functions {
		pct, err := acsassert.FuncCoverage(cover, fn)
		if err != nil {
			t.Errorf("RED: %v", err)
			continue
		}
		if pct < 100.0 {
			t.Errorf("%s coverage = %.1f%%, want 100.0%%", fn, pct)
		}
	}
}

func assertEveryFunctionOfFileCovered(t *testing.T, cover, fileMarker string) {
	t.Helper()
	seen := 0
	for _, line := range strings.Split(cover, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.Contains(fields[0], fileMarker) {
			continue
		}
		seen++
		if last := fields[len(fields)-1]; last != "100.0%" {
			t.Errorf("function %s of %s is covered %s, want 100.0%%", fields[1], strings.Trim(fileMarker, "/:"), last)
		}
	}
	if seen == 0 {
		t.Errorf("RED: the cover output lists no function of %s", strings.Trim(fileMarker, "/:"))
	}
}

func TestC2287_001_ASealedCycleWithADirtyLaneAndAnOrphanProcessIsCleanedAtOnceAfterASavedAndVerifiedBundle(t *testing.T) {
	sealedGCTests(t, "./internal/gc",
		"TestSweepSealedCycle_ASealedDirtyLaneWithAnOrphanIsCleanedAtOnce",
		"TestSweepSealedCycle_TheLaneIsSavedAndTheBundleVerifiedBeforeTheLaneAndBranchGo",
		"TestSweepSealedCycle_TheStalePointersOfTheSealedRunDoNotHoldItsLane",
		"TestSweepSealedCycle_ABranchWhoseLaneIsGoneIsReleasedAndNoOtherBranchIs",
		"TestSweepSealedCycle_AFailedProcessListingStillReleasesTheLaneAndIsReported")
}

func TestC2287_002_ACycleThatSealsIntoAPauseOrAContinuationOrALiveRunKeepsItsLaneAndBranch(t *testing.T) {
	sealedGCTests(t, "./internal/gc",
		"TestSweepSealedCycle_ACycleThatSealsIntoAPauseKeepsItsLaneBranchAndProcesses",
		"TestSweepSealedCycle_ASequentialCycleThatSealsIntoAPauseKeepsItsLane",
		"TestSweepSealedCycle_ACycleThatSealsIntoAContinuationKeepsItsLaneBranchAndProcesses",
		"TestSweepSealedCycle_ACycleWithALiveRunKeepsItsLane")
}

func TestC2287_003_ALiveLaneOfAnotherCycleIsNeverTouchedAndARejectedBundleKeepsTheLane(t *testing.T) {
	sealedGCTests(t, "./internal/gc",
		"TestSweepSealedCycle_TheLiveLaneOfAnotherCycleIsNeverTouched",
		"TestSweepSealedCycle_TheLanePointedAtByThePlaneMarkerOfAnotherCycleIsNeverTouched",
		"TestSweepSealedCycle_ARejectedBundleKeepsTheLaneAndTheBranch",
		"TestSweepSealedCycle_ACycleWithNoLaneAndNoBranchIsAQuietNoop")
}

func TestC2287_004_TheSequentialLoopAndTheResumeBatchBothReachTheSweepWhenTheirCycleSeals(t *testing.T) {
	sealedGCTests(t, "./cmd/evolve",
		"TestDispatchSequentialCycle_ASealedCycleHasItsDirtyLaneBranchAndOrphanCleanedAtOnce",
		"TestDispatchSequentialCycle_TheGCModeGovernsTheSealedCycleSweep",
		"TestDispatchSequentialCycle_TheGCModeGovernsTheSealedCycleSweep/shadow",
		"TestDispatchSequentialCycle_TheGCModeGovernsTheSealedCycleSweep/off",
		"TestRunResumeBatch_AResumedCycleThatSealsHasItsLaneBranchAndOrphanCleanedAtOnce",
		"TestDispatchSequentialCycle_ACycleThatSealsIntoAPauseKeepsItsLaneBranchAndProcesses",
		"TestDispatchSequentialCycle_TheLiveLaneOfAnotherCycleIsNotTouched")
}

func TestC2287_005_AFleetLaneCycleThatSealsThroughEvolveCycleRunHasItsDirtyLaneBranchAndOrphanCleanedAtOnce(t *testing.T) {
	sealedGCTests(t, "./cmd/evolve",
		"TestRunCycleRun_ASealedFleetLaneCycleHasItsDirtyLaneBranchAndOrphanCleanedAtOnce",
		"TestRunCycleRun_TheGCModeGovernsTheSealedFleetLaneSweep",
		"TestRunCycleRun_TheGCModeGovernsTheSealedFleetLaneSweep/shadow",
		"TestRunCycleRun_TheGCModeGovernsTheSealedFleetLaneSweep/off",
		"TestRunCycleRun_TheLiveLaneOfAnotherCycleIsNotTouchedWhenAFleetLaneCycleSeals")
}

func TestC2287_006_EveryFunctionOfTheGCSweepAndEachSharedExtractionTheSweepCallsIsFullyCovered(t *testing.T) {
	cover := coverFunctions(t, "./internal/gc", gcSweepTests)
	assertEveryFunctionOfFileCovered(t, cover, "/internal/gc/cycleseal.go:")
	assertFullyCovered(t, cover, "runLive", "reapProcesses", "dropRef")
}

func TestC2287_007_TheLoopCallerOfTheSweepIsFullyCoveredByTheTestsOfEachPathThatSealsACycle(t *testing.T) {
	cover := coverFunctions(t, "./cmd/evolve", cmdSweepTests)
	assertEveryFunctionOfFileCovered(t, cover, "/cmd/evolve/cmd_loop_sealed_gc.go:")
}

func TestC2287_008_TheBuildExplanationThatThePullRequestCarriesNamesTheDecisionTheStepAndThePlan(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docs, err := filepath.Glob(filepath.Join(root, "docs", "explain", "builds", "cycle-2287-*.md"))
	if err != nil || len(docs) == 0 {
		t.Fatalf("RED: no build explanation docs/explain/builds/cycle-2287-*.md, the text that the pull request carries (err=%v)", err)
	}
	for _, id := range []string{"D12", "X10", planPath} {
		if !acsassert.FileMatchesRegex(t, docs[0], `(^|[^A-Za-z0-9])`+regexp.QuoteMeta(id)+`([^A-Za-z0-9]|$)`) {
			t.Errorf("RED: %s does not name %s", filepath.Base(docs[0]), id)
		}
	}
	rel, err := filepath.Rel(root, docs[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", rel); code == 0 {
		t.Errorf("RED: %s is gitignored and is dropped at ship", rel)
	}
}

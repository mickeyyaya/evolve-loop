//go:build acs

package cycle2271

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	planPath       = "docs/plans/gc-closes-the-legacy-gap-2026-11.md"
	gcSweepFile    = "/internal/gc/cycleseal.go:"
	cmdSweepFile   = "/cmd/evolve/cmd_loop_sealed_gc.go:"
	gcSweepEntry   = "SweepSealedCycle"
	cmdSweepEntry  = "sweepSealedCycleGC"
	cmdCoverFilter = "TestDispatchSequentialCycle_|TestRunResumeBatch_AResumedCycleThatSeals|SealedCycle"
)

func requireTests(t *testing.T, pkg string, names ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: pkg,
		Pattern: "^(" + strings.Join(parentTests(names), "|") + ")$",
		Names:   names,
	})
}

func parentTests(names []string) []string {
	parents := make([]string, len(names))
	for i, name := range names {
		parents[i], _, _ = strings.Cut(name, "/")
	}
	return slices.Compact(parents)
}

func coverFunc(t *testing.T, pkg, run string) string {
	t.Helper()
	moduleDir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatalf("module dir: %v", err)
	}
	profile := filepath.Join(t.TempDir(), "seal.cov")
	args := []string{"-C", moduleDir, "test", "-count=1", "-coverprofile=" + profile}
	if run != "" {
		args = append(args, "-run", run)
	}
	testOut, testErr, code, runErr := acsassert.SubprocessOutput("go", append(args, pkg)...)
	if runErr != nil || code != 0 {
		t.Fatalf("go test %s exit=%d err=%v\n%s", pkg, code, runErr, acsassert.OutputTail(testOut+testErr, 40))
	}
	coverOut, coverErr, code, runErr := acsassert.SubprocessOutput("go", "-C", moduleDir, "tool", "cover", "-func="+profile)
	if runErr != nil || code != 0 {
		t.Fatalf("go tool cover exit=%d err=%v\n%s", code, runErr, acsassert.OutputTail(coverOut+coverErr, 40))
	}
	return coverOut
}

func assertFileFullyCovered(t *testing.T, coverOut, fileMarker, entry string) {
	t.Helper()
	pct, err := acsassert.FuncCoverage(coverOut, entry)
	if err != nil {
		t.Fatalf("RED: %v", err)
	}
	if pct < 100.0 {
		t.Errorf("%s coverage = %.1f%%, want 100.0%%", entry, pct)
	}
	seen := 0
	for _, line := range strings.Split(coverOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.Contains(fields[0], fileMarker) {
			continue
		}
		seen++
		got, err := strconv.ParseFloat(strings.TrimSuffix(fields[len(fields)-1], "%"), 64)
		if err != nil || got < 100.0 {
			t.Errorf("function %s of %s is covered %s, want 100.0%%", fields[1], strings.Trim(fileMarker, "/:"), fields[len(fields)-1])
		}
	}
	if seen == 0 {
		t.Errorf("RED: the cover output lists no function of %s", strings.Trim(fileMarker, "/:"))
	}
}

func TestC2271_001_ASealedDirtyLaneWithAnOrphanIsCleanedAtOnceAfterASavedAndVerifiedBundle(t *testing.T) {
	requireTests(t, "./internal/gc",
		"TestSweepSealedCycle_ASealedDirtyLaneWithAnOrphanIsCleanedAtOnce",
		"TestSweepSealedCycle_TheLaneIsSavedAndTheBundleVerifiedBeforeTheLaneAndBranchGo",
		"TestSweepSealedCycle_TheStalePointersOfTheSealedRunDoNotHoldItsLane",
		"TestSweepSealedCycle_ABranchWhoseLaneIsGoneIsReleasedAndNoOtherBranchIs",
		"TestSweepSealedCycle_AFailedProcessListingStillReleasesTheLaneAndIsReported")
}

func TestC2271_002_ACycleThatSealsIntoAPauseOrAContinuationOrALiveRunKeepsItsLaneAndBranch(t *testing.T) {
	requireTests(t, "./internal/gc",
		"TestSweepSealedCycle_ACycleThatSealsIntoAPauseKeepsItsLaneBranchAndProcesses",
		"TestSweepSealedCycle_ASequentialCycleThatSealsIntoAPauseKeepsItsLane",
		"TestSweepSealedCycle_ACycleThatSealsIntoAContinuationKeepsItsLaneBranchAndProcesses",
		"TestSweepSealedCycle_ACycleWithALiveRunKeepsItsLane")
}

func TestC2271_003_ALiveLaneOfAnotherCycleAndAFailedBundleAreNeverTouched(t *testing.T) {
	requireTests(t, "./internal/gc",
		"TestSweepSealedCycle_TheLiveLaneOfAnotherCycleIsNeverTouched",
		"TestSweepSealedCycle_TheLanePointedAtByThePlaneMarkerOfAnotherCycleIsNeverTouched",
		"TestSweepSealedCycle_ARejectedBundleKeepsTheLaneAndTheBranch",
		"TestSweepSealedCycle_ACycleWithNoLaneAndNoBranchIsAQuietNoop")
}

func TestC2271_004_BothLoopCallersOfACycleEndReachTheSweepAndTheGCModeAndThePauseAndALiveLaneGovernIt(t *testing.T) {
	requireTests(t, "./cmd/evolve",
		"TestDispatchSequentialCycle_ASealedCycleHasItsDirtyLaneBranchAndOrphanCleanedAtOnce",
		"TestDispatchSequentialCycle_TheGCModeGovernsTheSealedCycleSweep",
		"TestDispatchSequentialCycle_TheGCModeGovernsTheSealedCycleSweep/shadow",
		"TestDispatchSequentialCycle_TheGCModeGovernsTheSealedCycleSweep/off",
		"TestRunResumeBatch_AResumedCycleThatSealsHasItsLaneBranchAndOrphanCleanedAtOnce",
		"TestDispatchSequentialCycle_ACycleThatSealsIntoAPauseKeepsItsLaneBranchAndProcesses",
		"TestDispatchSequentialCycle_TheLiveLaneOfAnotherCycleIsNotTouched")
}

func TestC2271_008_TheFreshLeaseThatTheFinishedRunLeavesInTheLoopProcessDoesNotHoldTheSweepAndAForeignOrPausedHoldStillDoes(t *testing.T) {
	requireTests(t, "./internal/gc",
		"TestSweepSealedCycle_TheFreshLeaseOfTheFinishedRunOwnedByTheSweepingProcessDoesNotHoldItsLane",
		"TestSweepSealedCycle_TheLeaseOfAnotherLiveProcessStillHoldsTheLaneWhileThisProcessIsAlsoAlive",
		"TestSweepSealedCycle_TheLeaseOwnedByTheSweepingProcessOfAPausedCycleStillHoldsItsLane",
		"TestSweepSealedCycle_ACycleWithALiveRunKeepsItsLane")
}

func TestC2271_009_BothLoopCallersCleanASealedCycleWhoseOrchestratorLeftALiveLeaseUnderTheProductionPidCheck(t *testing.T) {
	requireTests(t, "./cmd/evolve",
		"TestDispatchSequentialCycle_ASealedCycleWhoseOrchestratorLeftAFreshLeaseOwnedByTheLoopIsStillCleanedAtOnce",
		"TestRunResumeBatch_AResumedCycleWhoseOrchestratorLeftAFreshLeaseOwnedByTheLoopIsStillCleanedAtOnce",
		"TestDispatchSequentialCycle_ASealedCycleWhoseFreshLeaseIsOwnedByAnotherLiveProcessKeepsItsLane")
}

func TestC2271_005_EveryFunctionOfTheGCSweepFileIsFullyCovered(t *testing.T) {
	assertFileFullyCovered(t, coverFunc(t, "./internal/gc", ""), gcSweepFile, gcSweepEntry)
}

func TestC2271_006_EveryFunctionOfTheLoopSweepFileIsFullyCovered(t *testing.T) {
	assertFileFullyCovered(t, coverFunc(t, "./cmd/evolve", cmdCoverFilter), cmdSweepFile, cmdSweepEntry)
}

func TestC2271_007_TheDecisionIdsThePullRequestMustNameAreRowsOfTheLinkedPlan(t *testing.T) {
	// acs-predicate: config-check
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(planPath)))
	if err != nil {
		t.Fatalf("read the plan: %v", err)
	}
	plan := string(raw)
	for id, marker := range map[string]string{"D12": "When cycle N seals", "D13": "At the wave end"} {
		row := regexp.MustCompile(`(?m)^\| ` + id + ` \|(.*)$`).FindStringSubmatch(plan)
		if row == nil {
			t.Errorf("the plan has no decision row %s, so a pull request cannot name it", id)
			continue
		}
		if !strings.Contains(row[1], marker) {
			t.Errorf("row %s of the plan does not hold %q", id, marker)
		}
	}
	if !regexp.MustCompile(`(?m)^\| X10 \| The cycle-end step of D12`).MatchString(plan) {
		t.Errorf("the plan has no X10 row for the cycle-end step of D12")
	}
}

//go:build acs

package cycle2321

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goTests(t *testing.T, pkg string, names ...string) {
	t.Helper()
	moduleDir := filepath.Join(acsassert.RepoRoot(t), "go")
	pattern := "^(" + strings.Join(names, "|") + ")$"
	out, stderr, code, err := acsassert.SubprocessOutput("go", "-C", moduleDir, "test", "-count=1", "-v", "-tags", "integration", "-run", pattern, pkg)
	if err != nil || code != 0 {
		t.Fatalf("go test -run %s %s exited %d (err=%v)\n%s", pattern, pkg, code, err, acsassert.OutputTail(out+stderr, 40))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" (") {
			t.Errorf("%s did not run and pass in %s:\n%s", name, pkg, acsassert.OutputTail(out, 40))
		}
	}
}

func TestC2321_001_EvolveWaveNextEmitsWaveStartedAndWaveEndedThroughTheProductionPath(t *testing.T) {
	goTests(t, "./cmd/evolve",
		"TestWaveNext_EmitsWaveStarted",
		"TestWaveNext_EmitsWaveEndedForTheWaveItCloses",
		"TestWaveNext_TheFirstWaveEndsNothing",
		"TestWaveNext_ADryRunAndAFailedBoundaryEmitNothing")
}

func TestC2321_003_CIWatchEmitsCICompletedAndARedRunWarnsWithCIRunRed(t *testing.T) {
	goTests(t, "./internal/ciwatch",
		"TestCIWatch_EmitsCICompletedAndARedRunWarns",
		"TestCIWatch_AWatchThatNeverCompletesEmitsNothing",
		"TestCIWatch_TheEventSurvivesAFailedEscalation",
		"TestCIWatch_ANilCenterIsTheNullObject")
}

func TestC2321_004_TheEvolveCIWatchVerbPublishesItsEventsToTheChannels(t *testing.T) {
	goTests(t, "./cmd/evolve",
		"TestCIWatchVerb_PublishesCICompletedForEachWatchedWorkflow",
		"TestCIWatchVerb_ARedRunReachesTheErrorsChannelWithItsCode",
		"TestCIWatchVerb_ATagWatchPublishesOneEventForEachReleaseWorkflow")
}

func TestC2321_005_TheFiveKindsTheCIModuleAndTheRedRunCodeAreInTheClosedSets(t *testing.T) {
	goTests(t, "./internal/signalcenter",
		"TestRegistry_NewKindsAndCodesAreRegistered",
		"TestRegistry_TheRedRunCodeIsOwnedByTheCIModuleWithADoc")
}

func TestC2321_006_TheLegacyProjectStreamKeepsItsAllowListOfKinds(t *testing.T) {
	goTests(t, "./cmd/evolve", "TestLegacyProjectRecord_AllowsOnlyTheKindsThatExistedBeforeTheChannels")
}

func TestC2321_002_InboxClaimedAndReleasedAreEmittedByTheClaimAndReleasePathsTheLoopRuns(t *testing.T) {
	goTests(t, "./internal/inboxmover",
		"TestInboxLifecycle_EmitsClaimedAndReleased",
		"TestInboxLifecycle_ClaimPendingEmitsOneClaimedForEachItemItClaims",
		"TestInboxLifecycle_ARefusedClaimEmitsNoClaimedEvent",
		"TestInboxLifecycle_AReleaseThatMovesNothingEmitsNoReleasedEvent",
		"TestInboxLifecycle_AFailedReleaseEmitsNoReleasedEvent",
		"TestInboxLifecycle_ReleaseStaleClaimsEmitsOneReleasedForEachClaimItEnds",
		"TestInboxLifecycle_ReleaseClaimKeepingAndADuplicateRemovalAreReleases",
		"TestInboxLifecycle_ANilCenterIsTheNullObject")
	goTests(t, "./cmd/evolve", "TestInboxLifecycle_TheHostClaimerPublishesClaimedThroughTheRootCenter")
	goTests(t, "./internal/loopwave", "TestClaims_ReleasePublishesInboxReleasedThroughTheEnginesCenter")
}

func TestC2321_007_CIWatchFailsClosedWhenTheRootOrTheRunLookupFails(t *testing.T) {
	goTests(t, "./cmd/evolve",
		"TestRunCIWatch_AWorkingDirectoryFaultExitsIOWhenNoRootIsGiven",
		"TestRunCIWatch_AFailedRunLookupExitsIOAndNamesTheWorkflow")
}

func fullCoverage(t *testing.T, pkg, runPattern string, funcs ...string) {
	t.Helper()
	moduleDir := filepath.Join(acsassert.RepoRoot(t), "go")
	profile := filepath.Join(t.TempDir(), "cover.out")
	out, stderr, code, err := acsassert.SubprocessOutput("go", "-C", moduleDir, "test", "-count=1", "-tags", "integration", "-coverprofile", profile, "-run", runPattern, pkg)
	if err != nil || code != 0 {
		t.Fatalf("go test -cover %s exited %d (err=%v)\n%s", pkg, code, err, acsassert.OutputTail(out+stderr, 40))
	}
	report, stderr, code, err := acsassert.SubprocessOutput("go", "-C", moduleDir, "tool", "cover", "-func="+profile)
	if err != nil || code != 0 {
		t.Fatalf("go tool cover exited %d (err=%v)\n%s", code, err, stderr)
	}
	for _, fn := range funcs {
		pct, perr := acsassert.FuncCoverage(report, fn)
		if perr != nil {
			t.Errorf("%s: %v", fn, perr)
			continue
		}
		if pct != 100 {
			t.Errorf("%s in %s has %.1f%% coverage, want 100%%", fn, pkg, pct)
		}
	}
}

func TestC2321_008_EachNewAndChangedFunctionOfTheLaneHasFullCoverage(t *testing.T) {
	fullCoverage(t, "./cmd/evolve", "CIWatch|RunCI", "runCIWatch", "watchWorkflows")
	fullCoverage(t, "./internal/ciwatch", ".", "Watch")
	fullCoverage(t, "./internal/inboxmover", ".", "ClaimDispatchable", "ClaimPending", "releaseHeld", "ReleaseStaleClaims", "ReleaseClaim", "ReleaseClaimKeeping")
}

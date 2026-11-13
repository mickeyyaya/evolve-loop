//go:build acs

package cycle2353

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const modulePath = "github.com/mickeyyaya/evolve-loop/go/"

func requireTestsPass(t *testing.T, pkg string, extra []string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	args := append([]string{"test", "-v"}, extra...)
	args = append(args, "-run", pattern, modulePath+pkg)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
	if code != 0 {
		t.Fatalf("RED: go %v exited %d\n%s", args, code, acsassert.OutputTail(stdout+stderr, 30))
	}
	for _, name := range names {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Fatalf("RED: %s never ran to PASS in %s (renamed or filtered out)\n%s", name, pkg, acsassert.OutputTail(stdout+stderr, 30))
		}
	}
}

func TestC2353_001_TheLaneEntryPointAdoptsOnceWhateverTheScheduling(t *testing.T) {
	requireTestsPass(t, "cmd/evolve", []string{"-cpu", "1,4", "-count=25"},
		"TestLaneResume_TwoLanesOfOneScopeAdoptAPausedCycleExactlyOnce")
}

func TestC2353_002_ALaneThatFindsTheLeasedCycleAfterTheWinnerStartsFresh(t *testing.T) {
	requireTestsPass(t, "cmd/evolve", []string{"-cpu", "1,4", "-count=25"},
		"TestLaneResume_ALaneThatArrivesAfterTheWinnerLeasedTheCycleStartsFresh")
}

func TestC2353_003_TheOperatorResumeAndTheRunCycleWiringKeepTheirRecorders(t *testing.T) {
	requireTestsPass(t, "cmd/evolve", []string{"-count=1"},
		"TestLoopResume_TheOperatorResumeKeepsTheLeaseFreeRecorder",
		"TestRunCycleRun_TheLaneRecordsItsResumeThroughRecordLaneResume",
		"TestLaneResume_UnleaseWarnsWhenTheLeaseCannotBeReleased")
}

func TestC2353_004_TwoConcurrentLaneResumesAdmitExactlyOneAndALeasedWorkspaceIsRefused(t *testing.T) {
	requireTestsPass(t, "internal/checkpoint", []string{"-count=1"},
		"TestRecordLaneResume_TwoConcurrentLaneResumesOfOnePausedCycleAdmitExactlyOne",
		"TestRecordLaneResume_AFreshLeaseRefusesAndSpendsNoAttempt",
		"TestRecordLaneResume_ASpentBudgetTakesNoLease",
		"TestRecordResume_TheOperatorResumeIgnoresAFreshLeaseAndTakesNone")
}

func TestC2353_005_TakeResumeLeaseRefusesAFreshHolderAndTakesOverAnExpiredOne(t *testing.T) {
	requireTestsPass(t, "internal/core", []string{"-count=1"},
		"TestTakeResumeLease_AFreeWorkspaceIsLeasedToThisProcess",
		"TestTakeResumeLease_AFreshLeaseRefusesWithAnErrorThatNamesTheHolder",
		"TestTakeResumeLease_AnExpiredLeaseIsTakenOver",
		"TestTakeResumeLease_AFreshUnreadableLeaseRefusesTheResumeWithTheCause",
		"TestTakeResumeLease_AnAgedUnreadableLeaseIsTakenOver",
		"TestReleaseResumeLease_RemovesOnlyTheAdmissionLeaseOfThisProcess",
		"TestTakeResumeLease_AnUnwritableWorkspaceRefusesTheResume")
}

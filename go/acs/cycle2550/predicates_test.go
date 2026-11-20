//go:build acs

package cycle2550

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	bridgePkg        = "./internal/bridge"
	bridgeNoteRel    = "docs/architecture/packages/internal-bridge.md"
	lateEnterTest    = "TestVerifySubmitted_EnterIgnoredFor20sThenAcceptedEndsSubmittedAfterResend"
	defaultSchedTest = "TestVerifySubmitted_DefaultScheduleResendsAtTwoFiveTenTwentyThirtySeconds"
)

func runNamedTests(t *testing.T, pkg string, names ...string) {
	t.Helper()
	dir := acsassert.GoModuleDir(t)
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", dir, "test", "-count=1", "-v", "-run", pattern, pkg)
	if err != nil || code != 0 {
		t.Fatalf("RED: go test %s -run %s exit=%d err=%v\n%s", pkg, pattern, code, err, acsassert.OutputTail(stdout+stderr, 40))
	}
	for _, name := range names {
		if !strings.Contains(stdout, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass in %s\n%s", name, pkg, acsassert.OutputTail(stdout, 20))
		}
	}
}

func TestC2550_001_EnterIgnoredFor20sThenAcceptedEndsSubmittedAfterResend(t *testing.T) {
	runNamedTests(t, bridgePkg, lateEnterTest)
}

func TestC2550_002_FourConcurrentSeatsEachSubmitAfterALateEnterThreeRoundsInARow(t *testing.T) {
	runNamedTests(t, bridgePkg, "TestVerifySubmitted_FourConcurrentSeatsAllSubmitThreeRoundsInARow")
}

func TestC2550_003_ResendScheduleIsATableFedFromDepsAndStillWedgesWhenEnterIsNeverAccepted(t *testing.T) {
	runNamedTests(t, bridgePkg,
		defaultSchedTest,
		"TestVerifySubmitted_ResendScheduleComesFromDeps",
		"TestVerifySubmitted_ConfiguredScheduleStopsResendingOnceTheEnterIsAccepted")
}

func TestC2550_006_TheLastScheduledEnterIsObservedBeforeTheSeatIsCalledWedged(t *testing.T) {
	runNamedTests(t, bridgePkg,
		"TestVerifySubmitted_FinalScheduledEnterIsObservedBeforeGivingUp",
		"TestVerifySubmitted_AcceptedEnterIsObservedBeforeTheNextEnterIsSent",
		"TestVerifySubmitted_NeverAcceptedPaneIsObservedAfterTheLastEnterThenWedges",
		"TestVerifySubmitted_OutwaitsALateEnterUnderBootLoad")
}

// acs-predicate: config-check
func TestC2550_005_BridgePackageNoteRecordsTheResendRulesWithTheEvidence(t *testing.T) {
	root := acsassert.RepoRoot(t)
	note := filepath.Join(root, bridgeNoteRel)
	if !acsassert.FileExists(t, note) {
		t.Fatalf("RED: %s missing on disk", bridgeNoteRel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", bridgeNoteRel); code != 0 {
		t.Errorf("RED: %s untracked", bridgeNoteRel)
	}
	for _, needle := range []string{"submit_resend_schedule_s", "Deps.SubmitResendSchedule", "2, 5, 10, 20, 30", "submitted_after_resend", lateEnterTest, "readiness", "TestVerifySubmitted_FinalScheduledEnterIsObservedBeforeGivingUp"} {
		if !acsassert.FileContains(t, note, needle) {
			t.Errorf("RED: %s does not record %q", bridgeNoteRel, needle)
		}
	}
}

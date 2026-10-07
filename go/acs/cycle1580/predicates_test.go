//go:build acs

package cycle1580

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

const deliverablePkg = "github.com/mickeyyaya/evolve-loop/go/internal/deliverable"

func runBridgeTests(t *testing.T, pattern string) (ok bool, out string) {
	t.Helper()
	return runPkgTests(t, bridgePkg, pattern)
}

func runPkgTests(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC1580_001_TransientRegexResolvedOnceFromTheManifest(t *testing.T) {
	ok, out := runBridgeTests(t, "TestAutoResponder_TransientRegexResolvedAtConstruction|TestAutoResponder_TransientRegexIsFamilyAgnostic")
	if !ok {
		t.Errorf("AC-1 unmet: the transient pattern is not resolved from the launched CLI's manifest at construction\n%s", out)
	}
}

func TestC1580_003_SixtySecondDwellWithReset(t *testing.T) {
	ok, out := runBridgeTests(t, "TestRunTmuxREPL_TransientDwell_EnforceStopsBeforeTheArtifactReviewer|"+
		"TestRunTmuxREPL_TransientDwell_DoesNotFireBeforeSixtySeconds|"+
		"TestRunTmuxREPL_TransientDwell_ResetsOnNonMatchingFrame|"+
		"TestRunTmuxREPL_TransientPaneSkipsFullArtifactTimeout")
	if !ok {
		t.Errorf("AC-3 unmet: the 60s transient dwell tracker does not fire/hold/reset as specified\n%s", out)
	}
}

func TestC1580_004_BusyPaneIsNeverPreempted(t *testing.T) {
	ok, out := runBridgeTests(t, "TestRunTmuxREPL_TransientDwell_BusyPaneIsNeverPreempted")
	if !ok {
		t.Errorf("AC-4 unmet: a BUSY pane was fast-failed — the dwell must carry the same busy guard as fatalPaneVerdict\n%s", out)
	}
}

func TestC1580_005_StageDialAndDurableTelemetry(t *testing.T) {
	ok, out := runBridgeTests(t, "TestRunTmuxREPL_TransientDwell_ShadowObservesWithoutActing|"+
		"TestRunTmuxREPL_TransientDwell_EnforceRecordsFastFailed|"+
		"TestRunTmuxREPL_TransientDwell_OffStageIsLegacy")
	if !ok {
		t.Errorf("AC-5 unmet: the ADR-0044 stage dial and/or the would/did telemetry is not wired\n%s", out)
	}
}

func TestC1580_006_ReviewStopReusesTheCompletedBlock(t *testing.T) {
	ok, out := runBridgeTests(t, "TestRunTmuxREPL_TransientDwell_ReusesExistingExitAndArtifacts")
	if !ok {
		t.Errorf("AC-6 unmet: the shortcircuit does not route through the existing ReviewStop/!completed path\n%s", out)
	}
}

func TestC1580_007_RedispatchDelayScopedToTheShortcircuit(t *testing.T) {
	ok, out := runBridgeTests(t, "TestRunTmuxREPL_TransientDwell_EnforceDelaysRedispatch|"+
		"TestRunTmuxREPL_TransientDwell_NoDelayOnOrdinaryTimeout")
	if !ok {
		t.Errorf("AC-7 unmet: the re-dispatch delay is missing on the shortcircuit path (or leaked onto the ordinary one)\n%s", out)
	}
}

func TestC1580_008_ScoutReportChallengeTokenIsEnforcedNatively(t *testing.T) {
	ok, out := runPkgTests(t, deliverablePkg, "TestVerify_Scout_MissingChallengeToken_Violation|"+
		"TestVerify_Scout_WrongToken_Violation|"+
		"TestVerify_Scout_TokenEchoed_OK|"+
		"TestVerify_Scout_NoTokenFile_FailOpen|"+
		"TestContract_RequireChallengeToken_CoversTokenConsumingPhases")
	if !ok {
		t.Errorf("AC-R1 unmet: the scout challenge-token binding is not enforced at the pre-audit contract gate — it still fails open on persona prose\n%s", out)
	}
}

func TestC1580_009_ErroredCaptureDoesNotReanchorThePaneDelta(t *testing.T) {
	ok, out := runBridgeTests(t, "TestRunTmuxREPL_CaptureErrorDoesNotReanchorPaneDelta|"+
		"TestRunTmuxREPL_CaptureErrorStillCompletes")
	if !ok {
		t.Errorf("AC-R2 unmet: an errored CapturePane still re-anchors the pane delta and duplicates the live stream\n%s", out)
	}
}

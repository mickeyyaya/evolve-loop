package launchoutcome

import (
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func denial(neutral, cwd, guard, path string) signalcenter.Event {
	return signalcenter.Event{Code: core.CodeGuardDenied, Fields: map[string]string{
		core.DenialFieldGuard: guard, core.DenialFieldPath: path, core.DenialFieldCWD: cwd, core.DenialFieldNeutral: neutral,
	}}
}

func TestGuardRefusal_OnlyANeutralDenialUnderTheCellMatches(t *testing.T) {
	t.Parallel()
	cell := "/work/cell-a"
	for name, tc := range map[string]struct {
		events []signalcenter.Event
		want   Refusal
	}{
		"the cell itself":            {[]signalcenter.Event{denial("true", cell, "ship", "git push")}, Refusal{Guard: "ship", Path: "git push"}},
		"a path under the cell":      {[]signalcenter.Event{denial("true", cell+"/go/internal", "ship", "git commit")}, Refusal{Guard: "ship", Path: "git commit"}},
		"a trailing slash":           {[]signalcenter.Event{denial("true", cell+"/", "ship", "git push")}, Refusal{Guard: "ship", Path: "git push"}},
		"the first match wins":       {[]signalcenter.Event{denial("true", cell, "ship", "first"), denial("true", cell, "ship", "second")}, Refusal{Guard: "ship", Path: "first"}},
		"another cell":               {[]signalcenter.Event{denial("true", "/work/cell-b", "ship", "git push")}, Refusal{}},
		"a sibling with the prefix":  {[]signalcenter.Event{denial("true", cell+"-b", "ship", "git push")}, Refusal{}},
		"a family-bound guard":       {[]signalcenter.Event{denial("false", cell, "role", "/etc/x")}, Refusal{}},
		"another signal code":        {[]signalcenter.Event{{Code: "BRIDGE_EXIT_ARTIFACT_TIMEOUT", Fields: denial("true", cell, "ship", "p").Fields}}, Refusal{}},
		"no events":                  {nil, Refusal{}},
		"a neutral denial after one": {[]signalcenter.Event{denial("false", cell, "role", "a"), denial("true", cell, "ship", "b")}, Refusal{Guard: "ship", Path: "b"}},
	} {
		if got := GuardRefusal(tc.events, cell); got != tc.want {
			t.Errorf("%s: %+v, want %+v", name, got, tc.want)
		}
	}
	if got := GuardRefusal([]signalcenter.Event{denial("true", "/anything", "ship", "p")}, ""); got.Present() {
		t.Errorf("an unknown cell matches nothing: %+v", got)
	}
}

func TestRefusal_PresentAndTextNameTheGuardAndThePath(t *testing.T) {
	t.Parallel()
	if (Refusal{}).Present() || (Refusal{}).Text() != "" {
		t.Error("the zero refusal is absent and has no text")
	}
	r := Refusal{Guard: "ship", Path: "git push origin main"}
	if !r.Present() || !strings.Contains(r.Text(), "guard ship denied git push origin main") || strings.Contains(strings.ToLower(r.Text()), "quota") {
		t.Errorf("text %q must name the guard and the path, and never name quota", r.Text())
	}
}

func TestClassify_AStallAfterADenialIsGuardRefused(t *testing.T) {
	t.Parallel()
	r := Refusal{Guard: "ship", Path: "git push origin main"}
	out := (Attempt{Code: ExitArtifactTimeout, Stderr: "[claude-tmux] FAIL: completion never signalled\n", Refusal: r}).Classify()
	if out.CauseCode != core.CauseGuardRefused || out.Transient || !errors.Is(out.Err, core.ErrArtifactTimeout) || out.Signal != CodeExitArtifactTimeout {
		t.Fatalf("outcome %+v: cause guard_refused, the sentinel and the signal of exit 81 kept", out)
	}
}

func TestAttemptDetail_AGuardRefusalNamesTheGuardAndThePathAndAnyOtherCauseKeepsItsLine(t *testing.T) {
	t.Parallel()
	r := Refusal{Guard: "ship", Path: "git push origin main"}
	if got := (Attempt{Code: ExitArtifactTimeout, Stderr: "[bridge] noise\n", Refusal: r}).detail(core.CauseGuardRefused); got != r.Text() {
		t.Errorf("a refusal gives its own text, got %q", got)
	}
	if got := (Attempt{Code: ExitArtifactTimeout, Stderr: "[bridge] noise\n", Refusal: r}).detail("artifact_timeout"); got != causeLine(ExitArtifactTimeout, "[bridge] noise\n") || got == r.Text() {
		t.Errorf("another cause keeps the cause line, got %q", got)
	}
}

func TestClassify_ATypedInfrastructureCauseKeepsItsCodeAndItsChain(t *testing.T) {
	t.Parallel()
	r := Refusal{Guard: "ship", Path: "git push"}
	for _, cause := range []TimeoutCause{TimeoutContextCancelled, TimeoutPaneLost, TimeoutDetectorError, TimeoutSubmitWedged, TimeoutTransientUpstream, TimeoutReviewStop, TimeoutReviewPause} {
		stderr := "[bridge] " + ArtifactTimeoutMarker + "cause=" + string(cause) + " reason=\"x\"\n"
		out := (Attempt{Code: ExitArtifactTimeout, Stderr: stderr, Refusal: r}).Classify()
		if out.CauseCode != string(cause) {
			t.Errorf("%s: outcome %+v keeps the typed cause", cause, out)
		}
	}
}

func TestClassify_ADenialOverIncompleteAndPlainTimeoutsIsGuardRefused(t *testing.T) {
	t.Parallel()
	r := Refusal{Guard: "ship", Path: "git push"}
	for name, stderr := range map[string]string{"incomplete": "[bridge] " + ArtifactTimeoutMarker + "cause=incomplete reason=\"x\"\n", "no marker": ""} {
		if got := (Attempt{Code: ExitArtifactTimeout, Stderr: stderr, Refusal: r}).CauseCode(); got != core.CauseGuardRefused {
			t.Errorf("%s: %q", name, got)
		}
	}
}

func TestClassify_OnlyExit81TakesTheRefusal(t *testing.T) {
	t.Parallel()
	r := Refusal{Guard: "ship", Path: "git push"}
	for _, code := range []int{ExitREPLBootTimeout, ExitUnknownPrompt, ExitModelMismatch, ExitBadFlags} {
		if got := (Attempt{Code: code, Refusal: r}).CauseCode(); got == core.CauseGuardRefused {
			t.Errorf("exit %d must not become guard_refused", code)
		}
		if (Attempt{Code: code, Refusal: r}).CauseCode() != CauseCode(code, "") {
			t.Errorf("exit %d: a refusal changes nothing", code)
		}
	}
	if (Attempt{Code: ExitOK, Refusal: r}).CauseCode() != "" {
		t.Error("exit 0 has no cause")
	}
}

func TestClassify_AStallWithoutADenialKeepsArtifactTimeout(t *testing.T) {
	t.Parallel()
	if got := Classify(ExitArtifactTimeout, nil, "").CauseCode; got == core.CauseGuardRefused {
		t.Fatalf("no denial, no refusal: %q", got)
	}
}

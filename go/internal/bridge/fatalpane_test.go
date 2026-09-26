package bridge

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

func fatalEv(tail string, busy bool) StopEvent {
	return StopEvent{
		Kind: StopArtifactTimeout, Phase: "build", Cycle: 262,
		ElapsedS: 300, IntervalS: 300, Attempt: 0,
		Progressed: true, // the nudge-echo trap: a dead pane CAN read as progressed
		Busy:       busy,
		StdoutTail: tail,
	}
}

const fatalTail = "⏺ There's an issue with the selected model (auto). It may not exist or you may not have access to it."

func TestFatalPaneVerdict_EnforcePreemptsWithStop(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	v, preempted := fatalPaneVerdict(recovery.SeedDetector(), fatalEv(fatalTail, false), "enforce", nil, &buf, "[t]")
	if !preempted {
		t.Fatal("enforce + fatal pane + not busy must preempt the reviewer (this is the ~20-min maxExtends burn fix)")
	}
	if v.Action != ReviewStop {
		t.Errorf("action=%s, want stop", v.Action)
	}
	if !strings.Contains(v.Reason, string(recovery.CauseModelInvalid)) {
		t.Errorf("reason must carry the typed cause for the justification trail; got %q", v.Reason)
	}
}

func TestFatalPaneVerdict_ShadowLogsButDoesNotPreempt(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	_, preempted := fatalPaneVerdict(recovery.SeedDetector(), fatalEv(fatalTail, false), "shadow", nil, &buf, "[t]")
	if preempted {
		t.Fatal("shadow must be behavior-neutral — log only, legacy verdict decides")
	}
	out := buf.String()
	if !strings.Contains(out, "shadow") || !strings.Contains(out, string(recovery.CauseModelInvalid)) {
		t.Errorf("shadow must log the would-be fast-fail with its typed cause; got %q", out)
	}
}

func TestFatalPaneVerdict_BusyPaneNeverPreempted(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	_, preempted := fatalPaneVerdict(recovery.SeedDetector(), fatalEv(fatalTail, true), "enforce", nil, &buf, "[t]")
	if preempted {
		t.Fatal("a Busy pane must never be preempted — never kill a working agent, even on a fatal-looking tail")
	}
}

func TestFatalPaneVerdict_OffSkipsDetection(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		det   *recovery.FatalPaneDetector
		stage string
	}{
		{"off", recovery.SeedDetector(), "off"},
		{"zero_value_stage", recovery.SeedDetector(), ""},
		{"nil_detector", nil, "enforce"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			_, preempted := fatalPaneVerdict(tc.det, fatalEv(fatalTail, false), tc.stage, nil, &buf, "[t]")
			if preempted {
				t.Fatal("must not preempt")
			}
			if buf.Len() != 0 {
				t.Errorf("must not log (no detector consult); got %q", buf.String())
			}
		})
	}
}

func TestFatalPaneVerdict_ShadowBusySuppressed(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	_, preempted := fatalPaneVerdict(recovery.SeedDetector(), fatalEv(fatalTail, true), "shadow", nil, &buf, "[t]")
	if preempted {
		t.Fatal("shadow never preempts")
	}
	if buf.Len() != 0 {
		t.Errorf("busy pane must suppress the shadow log; got %q", buf.String())
	}
}

func TestFatalPaneVerdict_HealthyPaneNotPreempted(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	_, preempted := fatalPaneVerdict(recovery.SeedDetector(), fatalEv("⏺ Running go test ./... — 14 packages", false), "enforce", nil, &buf, "[t]")
	if preempted {
		t.Fatal("healthy pane must never preempt")
	}
}

func TestRecoveryStageFromEnv(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"", "shadow"},
		{"shadow", "shadow"},
		{"enforce", "enforce"},
		{"ENFORCE", "enforce"},
		{"off", "off"},
		{"bogus", "off"},
	}
	for _, tc := range cases {
		deps := Deps{RecoveryStage: tc.in}
		if got := recoveryStageFromEnv(deps); got != tc.want {
			t.Errorf("RecoveryStage=%q → %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFatalPaneStageOf(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"", "shadow"},
		{"shadow", "shadow"},
		{"enforce", "enforce"},
		{" ENFORCE ", "enforce"},
		{"off", "off"},
		{"enforec", "off"},
	}
	for _, tc := range cases {
		deps := Deps{RecoveryStage: "enforce", FatalPaneStage: tc.in}
		if got := fatalPaneStageOf(deps); got != tc.want {
			t.Errorf("FatalPaneStage=%q (RecoveryStage=enforce) → %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFatalPaneVerdict_EnforceCarriesTheTypedCause(t *testing.T) {
	t.Parallel()
	for tail, want := range map[string]recovery.TerminalCause{
		fatalTail: recovery.CauseModelInvalid,
		"user@host evolve-loop %\nzsh: command not found: Please\nuser@host evolve-loop %": recovery.CauseDeadShell,
	} {
		var buf bytes.Buffer
		v, preempted := fatalPaneVerdict(recovery.SeedDetector(), fatalEv(tail, false), "enforce", nil, &buf, "[t]")
		if !preempted || v.Cause != want {
			t.Errorf("tail %q: preempted=%v cause=%q, want a preempting verdict with cause %q", tail, preempted, v.Cause, want)
		}
	}
	var buf bytes.Buffer
	if v, preempted := fatalPaneVerdict(recovery.SeedDetector(), fatalEv(fatalTail, false), "shadow", nil, &buf, "[t]"); preempted || v.Cause != "" {
		t.Errorf("shadow never preempts and carries no cause: preempted=%v cause=%q", preempted, v.Cause)
	}
}

package bridge

// fatalpane_test.go — ADR-0044 C2 (Slice 2) RED tests: the fatal-pane
// fast-fail seam at the stop-review checkpoint.
//
// cycle-262 mechanism: a dead pane (codex self-update → bare zsh; claude
// --model auto boot error) never produces an artifact, but the bridge's own
// nudge text echoes into the pane, reads as "progress" next interval, and
// buys extension after extension — ~20 min per phase against maxExtends on a
// state that was fatal on sight. The fix consults the deterministic
// recovery.FatalPaneDetector BEFORE the reviewer at each checkpoint:
//
//   stage=off     → detector not consulted; byte-identical legacy flow
//   stage=shadow  → detect + log the would-be fast-fail; legacy verdict still
//                   decides (behavior-neutral soak; the DEFAULT)
//   stage=enforce → a fatal match on a non-Busy pane preempts the reviewer
//                   with ReviewStop; the wait exits this interval and the
//                   runner's exit-81 fallback chain takes over immediately
//
// A Busy pane is NEVER preempted regardless of stage — the prime directive of
// the stop-review layer (never kill a working agent) outranks fast-fail.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/recovery"
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
	// "off", the "" zero value, and a nil detector must all behave as off:
	// no preempt, no detector consult, no log — a zero-value call path must
	// never silently enable (or even observe for) a kill-path.
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
	// Busy outranks the detector in EVERY stage — shadow must not even log
	// for a visibly-working agent, or the soak trail fills with noise about
	// panes that merely mention a signature.
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

// TestRecoveryStageFromEnv pins the bridge-side stage resolution via
// Deps.RecoveryStage (policy-injected, ADR-0044): unset → shadow (the
// behavior-neutral default), a typo → off (never silently enabling a
// kill-path), explicit values normalized case-insensitively.
func TestRecoveryStageFromEnv(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"", "shadow"},
		{"shadow", "shadow"},
		{"enforce", "enforce"},
		{"ENFORCE", "enforce"},
		{"off", "off"},
		{"bogus", "off"}, // typo defaults to off, never to a kill-path
	}
	for _, tc := range cases {
		deps := Deps{RecoveryStage: tc.in}
		if got := recoveryStageFromEnv(deps); got != tc.want {
			t.Errorf("RecoveryStage=%q → %q, want %q", tc.in, got, tc.want)
		}
	}
}

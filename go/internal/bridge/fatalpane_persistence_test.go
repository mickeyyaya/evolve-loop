package bridge

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// healthyTail is ordinary working-agent output, the frame that follows a
// transient fatal-shaped render and must reset the streak.
const healthyTail = "⏺ Reading incident-report.md … done. Now writing the audit."

type gateObs struct {
	verdict   ReviewVerdict
	preempted bool
	outcomes  []interaction.Outcome
	stderr    string
}

func observeFatal(t *testing.T, g *fatalPaneGate, rec *interaction.Recorder, stage, tail string, busy bool) gateObs {
	t.Helper()
	var buf bytes.Buffer
	v, preempted := g.verdict(recovery.SeedDetector(), fatalEv(tail, busy), stage, rec, &buf, "[t]")
	return gateObs{verdict: v, preempted: preempted, outcomes: rec.Outcomes(), stderr: buf.String()}
}

func TestFatalPaneGate_ThresholdMirrorsExhaustionGuard(t *testing.T) {
	t.Parallel()
	if fatalPanePersistObservations != exhaustionPersistObservations {
		t.Errorf("fatalPanePersistObservations = %d, want %d (same persistence bar as the quota-wall guard)",
			fatalPanePersistObservations, exhaustionPersistObservations)
	}
	if fatalPanePersistObservations < 2 {
		t.Fatalf("threshold %d cannot discriminate transient from persistent — the gate would be a no-op", fatalPanePersistObservations)
	}
	if g := newFatalPaneGate(); g.threshold != fatalPanePersistObservations {
		t.Errorf("newFatalPaneGate().threshold = %d, want %d", g.threshold, fatalPanePersistObservations)
	}
}

func TestFatalPaneGate_TransientMatchDoesNotFastFail(t *testing.T) {
	t.Parallel()
	g := newFatalPaneGate()
	rec := interaction.NewRecorder(t.TempDir())

	first := observeFatal(t, g, rec, "enforce", fatalTail, false)
	if first.preempted {
		t.Fatalf("a SINGLE fatal-shaped frame preempted the reviewer — a working agent quoting a fatal signature is killed (the cycle-254/255/314/641 cardinal false-FAIL); verdict=%+v", first.verdict)
	}
	if len(first.outcomes) != 0 {
		t.Errorf("un-persisted match left C2 evidence %+v — the soak's fast_failed/would_fast_fail counts must only reflect gate-crossed observations", first.outcomes)
	}

	if healthy := observeFatal(t, g, rec, "enforce", healthyTail, false); healthy.preempted {
		t.Fatalf("healthy pane preempted: %+v", healthy.verdict)
	}

	again := observeFatal(t, g, rec, "enforce", fatalTail, false)
	if again.preempted {
		t.Fatalf("a non-matching observation did not RESET the streak — two NON-consecutive lone matches crossed the gate; verdict=%+v", again.verdict)
	}
	if len(again.outcomes) != 0 {
		t.Errorf("non-consecutive matches recorded C2 evidence: %+v", again.outcomes)
	}
}

func TestFatalPaneGate_PersistentFatalPaneStillFastFails(t *testing.T) {
	t.Parallel()
	g := newFatalPaneGate()
	rec := interaction.NewRecorder(t.TempDir())

	firedAt := 0
	var fired gateObs
	for i := 1; i <= fatalPanePersistObservations+2 && firedAt == 0; i++ {
		obs := observeFatal(t, g, rec, "enforce", fatalTail, false)
		if obs.preempted {
			firedAt, fired = i, obs
		}
	}
	if firedAt == 0 {
		t.Fatalf("a PERSISTENT fatal pane never fast-failed within %d observations — the gate broke the ADR-0044 C2 rescue path (cycle-262 burned ~40 min on exactly this state)", fatalPanePersistObservations+2)
	}
	if firedAt != fatalPanePersistObservations {
		t.Errorf("fast-fail fired at observation %d, want %d — the gate must add exactly the precedent's bounded latency, no more", firedAt, fatalPanePersistObservations)
	}
	if fired.verdict.Action != ReviewStop {
		t.Errorf("action = %s, want %s", fired.verdict.Action, ReviewStop)
	}
	if !strings.Contains(fired.verdict.Reason, string(recovery.CauseModelInvalid)) {
		t.Errorf("reason must still carry the typed cause for the justification trail; got %q", fired.verdict.Reason)
	}
	if len(fired.outcomes) != 1 || fired.outcomes[0].Result != "fast_failed" {
		t.Errorf("want exactly one fast_failed record for the would/did parity check; got %+v", fired.outcomes)
	}
}

func TestFatalPaneGate_ShadowEvidenceOnlyAfterPersistence(t *testing.T) {
	t.Parallel()
	g := newFatalPaneGate()
	rec := interaction.NewRecorder(t.TempDir())

	first := observeFatal(t, g, rec, "shadow", fatalTail, false)
	if len(first.outcomes) != 0 {
		t.Errorf("shadow recorded on an un-persisted match: %+v (would/did parity requires shadow to predict the GATED enforce action)", first.outcomes)
	}
	if first.stderr != "" {
		t.Errorf("shadow logged an un-persisted match: %q", first.stderr)
	}

	second := observeFatal(t, g, rec, "shadow", fatalTail, false)
	if second.preempted {
		t.Fatal("shadow must never preempt, gate-crossed or not")
	}
	if len(second.outcomes) != 1 || second.outcomes[0].Result != "would_fast_fail" {
		t.Fatalf("a gate-crossed shadow observation must record exactly one would_fast_fail; got %+v", second.outcomes)
	}
	if o := second.outcomes[0]; o.Kind != "fatal_pane_shadow" || o.Trigger != string(recovery.CauseModelInvalid) {
		t.Errorf("record must keep its shape + typed cause: %+v", o.Event)
	}
	if !strings.Contains(second.stderr, "shadow") || !strings.Contains(second.stderr, string(recovery.CauseModelInvalid)) {
		t.Errorf("gate-crossed shadow must still log the would-be fast-fail with its cause; got %q", second.stderr)
	}
}

func TestFatalPaneGate_BusyObservationResetsStreak(t *testing.T) {
	t.Parallel()
	g := newFatalPaneGate()
	rec := interaction.NewRecorder(t.TempDir())

	for i, obs := range []gateObs{
		observeFatal(t, g, rec, "enforce", fatalTail, false), // match
		observeFatal(t, g, rec, "enforce", fatalTail, true),  // BUSY → reset
		observeFatal(t, g, rec, "enforce", fatalTail, false), // lone match again
	} {
		if obs.preempted {
			t.Fatalf("observation %d preempted: a Busy checkpoint between two lone matches must reset the streak (busy = working agent, not a fatal pane); verdict=%+v", i+1, obs.verdict)
		}
	}
	if outs := rec.Outcomes(); len(outs) != 0 {
		t.Errorf("no observation crossed the gate, so nothing may be recorded; got %+v", outs)
	}
}

func TestFatalPaneGate_DisabledPathsNeverAccumulate(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		stage string
		nilD  bool
	}{
		{"off", "off", false},
		{"zero_value_stage", "", false},
		{"nil_detector", "enforce", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := newFatalPaneGate()
			rec := interaction.NewRecorder(t.TempDir())
			for i := 0; i < fatalPanePersistObservations+2; i++ {
				var buf bytes.Buffer
				det := recovery.SeedDetector()
				if tc.nilD {
					det = nil
				}
				if _, preempted := g.verdict(det, fatalEv(fatalTail, false), tc.stage, rec, &buf, "[t]"); preempted {
					t.Fatalf("observation %d preempted on a disabled path", i+1)
				}
				if buf.Len() != 0 {
					t.Errorf("disabled path logged %q (the detector must not even be consulted)", buf.String())
				}
			}
			if outs := rec.Outcomes(); len(outs) != 0 {
				t.Errorf("disabled path recorded %+v", outs)
			}
			if obs := observeFatal(t, g, rec, "enforce", fatalTail, false); obs.preempted {
				t.Fatalf("first enforce observation preempted — a streak accumulated while the path was disabled; verdict=%+v", obs.verdict)
			}
		})
	}

	t.Run("nil_gate_is_fail_safe", func(t *testing.T) {
		t.Parallel()
		var g *fatalPaneGate
		var buf bytes.Buffer
		rec := interaction.NewRecorder(t.TempDir())
		if _, preempted := g.verdict(recovery.SeedDetector(), fatalEv(fatalTail, false), "enforce", rec, &buf, "[t]"); preempted {
			t.Fatal("a nil gate must never preempt — an unwired call site must fail SAFE (fail over), never kill")
		}
		if outs := rec.Outcomes(); len(outs) != 0 {
			t.Errorf("nil gate recorded %+v", outs)
		}
	})
}

func TestFatalPaneGate_AgentDiffNoiseDoesNotBankStreak(t *testing.T) {
	t.Parallel()
	g := newFatalPaneGate()
	rec := interaction.NewRecorder(t.TempDir())

	diffTail := "❯ review the change\n+ chrome := \"There's an issue with the selected model\"\n+ det.handle(chrome)\n"
	for i := 0; i < fatalPanePersistObservations+2; i++ {
		if obs := observeFatal(t, g, rec, "enforce", diffTail, false); obs.preempted {
			t.Fatalf("agent-diff frame %d preempted — the gate is detecting on the RAW pane (quoted signatures count as fatal)", i)
		}
	}
	if g.streak != 0 {
		t.Fatalf("streak = %d after %d agent-diff-only frames, want 0 — raw-pane observation banks quoted noise and lets the next real match cross with no persistence",
			g.streak, fatalPanePersistObservations+2)
	}

	if obs := observeFatal(t, g, rec, "enforce", fatalTail, false); obs.preempted {
		t.Fatal("a single real match after diff noise crossed the gate — the banked-noise saturation path is live")
	}
}

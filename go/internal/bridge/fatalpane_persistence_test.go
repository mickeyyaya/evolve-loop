package bridge

// fatalpane_persistence_test.go — RED tests for the fatal-pane persistence gate
// (cycle-1118). The exhaustion fast-fail is persistence-gated
// (exhaustion_persistence.go: a wall must be present on `threshold` CONSECUTIVE
// observations before it can kill the phase) precisely because the detectors match
// against the RAW captured pane, and a WORKING agent can render fatal-shaped TEXT
// into that pane — a cat/grep/diff of an incident report, a test fixture, a log
// excerpt. The fatal-pane seam (fatalpane.go) matches the same way — on pane
// substrings like "There's an issue with the selected model" / "Please restart
// Codex" — but fires on a SINGLE observation. Same false-FAIL class
// (cycle-254/255/314/641), unguarded.
//
// Contract under test — a loop-scoped gate wrapping the per-checkpoint decision:
//
//	gate := newFatalPaneGate()                      // ONE instance per checkpoint loop
//	v, preempted := gate.verdict(det, ev, stage, rec, stderr, pfx)
//
//  1. a fatal match must have persisted for fatalPanePersistObservations
//     CONSECUTIVE observations before it can preempt (enforce) or leave C2
//     evidence (shadow) — a transient frame never crosses;
//  2. any non-matching observation — healthy pane, or a Busy pane (busy outranks
//     the detector at every stage) — RESETS the streak;
//  3. a genuinely parked pane still fast-fails, at the threshold observation, with
//     the unchanged ADR-0044 C2 verdict — bounded extra latency, no regression of
//     the cycle-262 rescue path;
//  4. off / "" / nil detector never observe at all: a disabled path must not
//     silently accumulate a streak that a later stage flip could cash in.
//
// fatalPaneVerdict's OWN signature and single-observation semantics are unchanged
// (fatalpane_test.go / fatalpane_durable_test.go stay green unmodified) — the gate
// is additive state around the call, owned by the checkpoint loop the way
// checkpointExhaustGate is.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// healthyTail is ordinary working-agent output — the frame that follows a
// transient fatal-shaped render and must reset the streak.
const healthyTail = "⏺ Reading incident-report.md … done. Now writing the audit."

// gateObs drives one observation through a gate and returns the outcome plus the
// evidence surfaces (durable records + stderr) that observation produced.
type gateObs struct {
	verdict   ReviewVerdict
	preempted bool
	outcomes  []interaction.Outcome
	stderr    string
}

// observeFatal runs one gate.verdict observation with a real recorder and a real
// stderr buffer, so the test can assert on BOTH the decision and the C2 evidence
// trail (the two things the AC constrains).
func observeFatal(t *testing.T, g *fatalPaneGate, rec *interaction.Recorder, stage, tail string, busy bool) gateObs {
	t.Helper()
	var buf bytes.Buffer
	v, preempted := g.verdict(recovery.SeedDetector(), fatalEv(tail, busy), stage, rec, &buf, "[t]")
	return gateObs{verdict: v, preempted: preempted, outcomes: rec.Outcomes(), stderr: buf.String()}
}

// TestFatalPaneGate_ThresholdMirrorsExhaustionGuard — the gate must require the
// same number of consecutive observations as the precedent it copies; a threshold
// of 1 is the un-gated behavior wearing a gate's name.
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

// TestFatalPaneGate_TransientMatchDoesNotFastFail — THE class fix (negative axis;
// mirrors TestExhaustion_TransientWallTextDoesNotFastFail). A working agent that
// renders fatal-shaped text for ONE checkpoint and scrolls it away must never be
// preempted, and must leave NO "fast_failed" record behind. The streak resets on
// the healthy frame, so a later lone match cannot cash in the earlier one.
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

	// Post-reset lone match: the earlier observation must NOT still be banked.
	again := observeFatal(t, g, rec, "enforce", fatalTail, false)
	if again.preempted {
		t.Fatalf("a non-matching observation did not RESET the streak — two NON-consecutive lone matches crossed the gate; verdict=%+v", again.verdict)
	}
	if len(again.outcomes) != 0 {
		t.Errorf("non-consecutive matches recorded C2 evidence: %+v", again.outcomes)
	}
}

// TestFatalPaneGate_PersistentFatalPaneStillFastFails — the regression bound on
// the cycle-262 rescue path: a pane parked in a fatal state (present every
// checkpoint) must still fast-fail, at the threshold observation, with the
// unchanged ADR-0044 C2 verdict — and record fast_failed exactly ONCE (an inflated
// C2 count breaks the R8.5 would/did parity check).
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

// TestFatalPaneGate_ShadowEvidenceOnlyAfterPersistence — shadow must PREDICT
// enforce. If shadow logged/recorded would_fast_fail on a transient frame while
// enforce (gated) would not have acted, the R8.5 would/did parity check compares
// two different semantics and the soak reads as a behavior change that never
// happened.
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

// TestFatalPaneGate_BusyObservationResetsStreak — edge axis. Busy outranks the
// detector at every stage (never kill a working agent), so a Busy checkpoint is a
// NON-match for streak purposes: it must reset, not merely be skipped. Otherwise
// fatal-shaped text on either side of a visibly-working checkpoint accumulates
// into a kill.
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

// TestFatalPaneGate_DisabledPathsNeverAccumulate — a disabled classification path
// must not bank a streak that a later stage flip cashes in on its first enforce
// checkpoint. off / "" / nil-detector observe nothing; a nil gate is fail-safe
// (never preempt).
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
			// The flip: no streak may have been banked while disabled.
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

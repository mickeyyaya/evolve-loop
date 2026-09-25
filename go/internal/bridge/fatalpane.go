package bridge

// fatalpane.go — ADR-0044 C2: the fatal-pane fast-fail seam at the
// stop-review checkpoint (the bridge half of the Phase Recovery Pipeline's
// Analyze stage; the registry itself lives in internal/recovery, the single
// recovery owner).
//
// cycle-262 burned ~40 min waiting out the maxExtends backstop on two
// self-describing fatal pane states — the pane literally said "There's an
// issue with the selected model" / "Please restart Codex", but nothing read
// it. Worse, the bridge's own one-shot nudge echoed into the dead pane and
// counted as "progress" the next interval, buying extensions for a REPL that
// no longer existed. This seam consults the deterministic
// recovery.FatalPaneDetector BEFORE the StopReviewer each checkpoint, so a
// known-fatal state exits the wait in ONE interval and hands the phase to
// the runner's exit-81 fallback chain (which is exactly what rescued the
// cycle-262 build — 20 minutes too late).
//
// Stage discipline (recovery.fatal_pane — this seam's OWN dial since F27,
// split out of the whole-program PhaseRecovery dial the way SpineFloor was):
//
//	off     → detector not consulted; byte-identical legacy flow
//	shadow  → detect + log the would-be fast-fail; legacy verdict decides
//	          (the soak stage; also what an unwired Deps resolves to)
//	enforce → a fatal match on a non-Busy pane preempts the reviewer (the
//	          policy DEFAULT since F27 — every shadow match on record was a
//	          dead pane that then idled 900-1200s)
//
// A Busy pane is never preempted regardless of stage: the stop-review
// layer's prime directive (never kill a working agent — see the cycle-254/255
// false-FAIL post-mortem in stopreview.go) outranks fast-fail.

import (
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// recoveryStageFromEnv resolves the bridge-side ADR-0044 phase recovery stage.
// The stage is injected by the orchestrator via Deps.RecoveryStage (policy-resolved);
// empty → channel.ResolveStage returns "shadow" (the behavior-neutral default).
// channel.ResolveStage is the single home of the resolution rule so the in-process
// and subprocess readers can never drift (ADR-0045 I6).
func recoveryStageFromEnv(deps Deps) string {
	return channel.ResolveStage(deps.RecoveryStage)
}

// fatalPaneObservation is what the wait loop reports about a preempting fatal
// verdict (F31): the typed cause, whether the session is NAMED (the driver's
// one resolution of the flag/env/profile session name — resolveSession) and
// the wait interval it actually used. The engine's fresh-session gate reads
// these facts from the driver instead of re-deriving them.
type fatalPaneObservation struct {
	cause     recovery.TerminalCause
	named     bool
	intervalS int
}

// observeFatalPane hands the observation to the call-local observer Launch
// installs — nil-safe, and silent for an empty cause.
func observeFatalPane(deps Deps, obs fatalPaneObservation) {
	if deps.onFatalPane != nil && obs.cause != "" {
		deps.onFatalPane(obs)
	}
}

// fatalPaneStageOf resolves the fatal-pane fast-fail's OWN stage (F27) through
// the same normalizer as the program dial — unset → shadow, typo → off — and
// reads ONLY Deps.FatalPaneStage, so the two dials can never borrow each other.
func fatalPaneStageOf(deps Deps) string {
	return channel.ResolveStage(deps.FatalPaneStage)
}

// fatalPaneClassifies reports whether the C2 classification path is live for
// this detector/stage pair.
//
// "" is treated as "off": fatalPaneStageOf never returns "" (unset →
// "shadow"), but a direct caller passing the zero value must not silently
// enable a kill-path. Same posture for a nil detector — Detect is
// nil-receiver-safe, but the safety belongs visibly at THIS boundary, not
// buried in the callee.
//
// Single-sourced because fatalPaneGate (fatalpane_persistence.go) must make the
// SAME call to decide whether to observe at all: if the two drifted, the gate
// would bank a persistence streak on a path this seam considers disabled, and a
// later stage flip would cash it in on its first enforce checkpoint.
func fatalPaneClassifies(det *recovery.FatalPaneDetector, stage string) bool {
	return det != nil && stage != "off" && stage != ""
}

// fatalPaneVerdict consults the fatal-pane registry for one stop-review
// checkpoint. It returns (verdict, true) when enforcement preempts the
// reviewer — the caller skips StopReviewer.Review and applies the verdict —
// or (zero, false) when the legacy flow decides (off stage, shadow stage,
// busy pane, or no match). Shadow logs the would-be action to stderr so the
// soak leaves an auditable trail without changing behavior.
func fatalPaneVerdict(det *recovery.FatalPaneDetector, ev StopEvent, stage string, rec *interaction.Recorder, stderr io.Writer, pfx string) (ReviewVerdict, bool) {
	if !fatalPaneClassifies(det, stage) {
		return ReviewVerdict{}, false
	}
	// Scan the agent-STRIPPED pane, not the raw tail. Until cycle-1117 this
	// seam read ev.StdoutTail directly while its twin one field away (the
	// exhaustion scan) read a stripped pane — so an agent EDITING the fatal
	// registry, its diff view rendering `Substr: "There's an issue with the
	// selected model"`, was fast-failed on its own edit buffer. The protect-list
	// comes FROM the live registry (det.Signatures()), so the echo half can
	// never suppress a signature the detector is looking for.
	cause, sig, ok := det.Detect(strippedForFatalPaneScan(ev.StdoutTail, ev.InjectedPrompt, det.Signatures()))
	if !ok || ev.Busy {
		return ReviewVerdict{}, false
	}
	// R8.3: the match is recorded DURABLY beside the other I1 records —
	// would_fast_fail at shadow, fast_failed at enforce — so the soak
	// reporter has C2 evidence to read and the post-flip parity check can
	// compare would vs did. stderr alone left the soak blind by construction.
	// DELIBERATE exception to interaction.go's "record at every stage"
	// principle: at off the DETECTOR itself is not consulted (the early
	// return above), so there is no observation to record — recording would
	// require enabling the very classification "off" exists to disable.
	record := func(result string) {
		if rec == nil {
			return
		}
		rec.Record(interaction.Outcome{Event: interaction.Event{
			Kind:    "fatal_pane_shadow",
			Phase:   ev.Phase,
			Cycle:   ev.Cycle,
			Trigger: string(cause),
			Payload: sig,
		}, Result: result})
	}
	if stage == "enforce" {
		record("fast_failed")
		return ReviewVerdict{
			Action: ReviewStop,
			Reason: fmt.Sprintf("fatal pane state (%s): matched %q — fast-fail instead of burning the maxExtends backstop (ADR-0044 C2)", cause, sig),
			Cause:  cause,
		}, true
	}
	record("would_fast_fail")
	fmt.Fprintf(stderr, "%s stop-review shadow: would fast-fail — fatal pane state (%s) matched %q (recovery.fatal_pane=shadow; legacy verdict decides)\n", pfx, cause, sig)
	return ReviewVerdict{}, false
}

package bridge

import (
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// channel.ResolveStage is the single home of this resolution rule, so
// in-process and subprocess readers cannot drift.
// See ADR-0045.
func recoveryStageFromEnv(deps Deps) string {
	return channel.ResolveStage(deps.RecoveryStage)
}

// fatalPaneObservation carries what the wait loop reports about a preempting
// fatal verdict; the engine's fresh-session gate reads these facts from the
// driver instead of re-deriving them.
type fatalPaneObservation struct {
	cause     recovery.TerminalCause
	named     bool
	intervalS int
}

func observeFatalPane(deps Deps, obs fatalPaneObservation) {
	if deps.onFatalPane != nil && obs.cause != "" {
		deps.onFatalPane(obs)
	}
}

func fatalPaneStageOf(deps Deps) string {
	return channel.ResolveStage(deps.FatalPaneStage)
}

// fatalPaneClassifies is the one gate fatalPaneVerdict and fatalPaneGate both
// call, so they can never disagree about whether the detector applies at all.
func fatalPaneClassifies(det *recovery.FatalPaneDetector, stage string) bool {
	return det != nil && stage != "off" && stage != ""
}

func fatalPaneVerdict(det *recovery.FatalPaneDetector, ev StopEvent, stage string, rec *interaction.Recorder, stderr io.Writer, pfx string) (ReviewVerdict, bool) {
	if !fatalPaneClassifies(det, stage) {
		return ReviewVerdict{}, false
	}
	cause, sig, ok := det.Detect(strippedForFatalPaneScan(ev.StdoutTail, ev.InjectedPrompt, det.Signatures()))
	if !ok || ev.Busy {
		return ReviewVerdict{}, false
	}
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

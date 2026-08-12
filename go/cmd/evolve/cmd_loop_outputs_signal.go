package main

// cmd_loop_outputs_signal.go — the loop's post-cycle phase-output survey,
// reported into the unified signal stream (abnormal-events.jsonl) beside the
// counter/verify/classification events. Operator directive: the survey is not
// a CLI-only view — the SYSTEM reports it, every cycle, to the same center a
// dashboard reads, and `evolve cycle outputs` is the drill-down.
//
// Thin adapter: reading goes through internal/phaseoutputs' shared loaders and
// every decision (gap, chain state, abnormality) is made in that pure layer —
// this file only sequences reads and hands the verdict to the event writer.

import (
	"fmt"
	"io"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseoutputs"
)

// emitPhaseOutputsSurvey surveys one finished cycle's workspace and appends
// the result to the unified signal stream. Best-effort like its sibling
// emitters — a failed survey must never take down the loop — but never
// silent: every skip and emit failure lands on stderr with the cycle number.
func emitPhaseOutputsSurvey(workspace string, cycle int, stderr io.Writer) {
	completed, err := phaseoutputs.LoadCompletedPhases(workspace)
	if err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: phase-outputs survey skipped for cycle %d: %v\n", cycle, err)
		return
	}
	listing, err := phaseoutputs.LoadListing(workspace)
	if err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: phase-outputs survey skipped for cycle %d: %v\n", cycle, err)
		return
	}
	survey := phaseoutputs.Survey(completed, listing)
	chain := phaseoutputs.CycleChainStatus(
		slices.Contains(completed, "audit"),
		phaseoutputs.LoadShadowReading(workspace, auditchain.ShadowRecordFile),
	)
	details, abnormal := phaseoutputs.Signal(survey, chain)
	if err := dispatchevents.NewWriter(workspace).EmitPhaseOutputsSurveyed(cycle, details, abnormal); err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: phase-outputs signal emit failed for cycle %d: %v\n", cycle, err)
	}
	fmt.Fprintf(stderr, "[loop] cycle %d %s\n", cycle, details)
}

package core

// phaseoutputs_signal.go — the per-cycle phase-output survey, emitted into the
// unified signal stream (abnormal-events.jsonl) at cycle finalize.
//
// This lives in CORE, not the loop CLI, because the first monitored wave
// proved the placement wrong the other way: fleet-dispatched lanes never
// traverse cmd_loop's single-loop post-cycle block, so cycles 1452/1453
// completed with zero phase-outputs-surveyed events — the exact silent
// non-reporting the survey exists to end. The finalize defers in RunCycle and
// RunCycleFromPhase run for every cycle on every dispatch topology, abort
// paths included, so the emission cannot be skipped by how the cycle was
// launched. A cycle that aborts and later resumes emits once per finalize —
// two events on one stream, both true at their moment; consumers take the
// LAST event per cycle (documented on EventPhaseOutputsSurveyed).
//
// Thin adapter: reading goes through internal/phaseoutputs' shared loaders and
// every decision (gap, chain state, abnormality) is made in that pure layer.

import (
	"fmt"
	"os"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseoutputs"
)

// emitPhaseOutputsSignal surveys one finished cycle's workspace and appends
// the result to the unified signal stream. Completed phases come from the
// caller's in-memory CycleState (the authority run.json merely mirrors);
// resolver is the caller's catalog-aware contract resolution so spec-derived
// phases (memo, minted) resolve their real report names. Best-effort — a
// failed survey must never take down a cycle — but never silent: every skip
// and emit failure lands on stderr with the cycle number.
func emitPhaseOutputsSignal(workspace string, cycle int, completed []string, resolver phasecontract.Resolver) {
	if workspace == "" {
		return
	}
	listing, err := phaseoutputs.LoadListing(workspace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN: phase-outputs survey skipped for cycle %d: %v\n", cycle, err)
		return
	}
	survey := phaseoutputs.Survey(completed, listing, resolver)
	chain := phaseoutputs.CycleChainStatus(
		slices.Contains(completed, "audit"),
		phaseoutputs.LoadShadowReading(workspace, auditchain.ShadowRecordFile),
	)
	details, abnormal := phaseoutputs.Signal(survey, chain)
	if err := dispatchevents.NewWriter(workspace).EmitPhaseOutputsSurveyed(cycle, details, abnormal); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN: phase-outputs signal emit failed for cycle %d: %v\n", cycle, err)
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d %s\n", cycle, details)
}

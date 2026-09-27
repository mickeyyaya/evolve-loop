package core

import (
	"fmt"
	"os"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseoutputs"
)

// emitPhaseOutputsSignal surveys a finished cycle's workspace and appends the
// result to the unified signal stream. Completed phases come from the
// caller's in-memory CycleState, the authority over run.json's mirror.
// Best-effort but never silent: every skip or emit failure lands on stderr
// with the cycle number.
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

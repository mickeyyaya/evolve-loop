package core

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeFleetRebaseConflict signalcenter.Code = "ORCHESTRATOR_FLEET_REBASE_CONFLICT"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeFleetRebaseConflict,
		"the fleet rebase of the ship recovery hit a merge conflict, and the cycle routes to the debugger (ADR-0143 W1b reads it for a lane that only the optimistic admission rule admitted); not a landing-conflict ship error, so the oversight does not count it again; WARN; fields.paths (the conflicted paths)")
}

func (o *Orchestrator) emitFleetRebaseConflict(cycle int, cs CycleState) {
	o.signals.Emit(signalcenter.Event{
		Cycle: cycle, RunID: cs.RunID, Phase: string(PhaseShip), Module: signalcenter.ModuleOrchestrator, Origin: "Orchestrator.recoverFromShipError",
		Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityWarn, Code: CodeFleetRebaseConflict,
		Reason: "fleet rebase conflict", Fields: map[string]string{"paths": strings.Join(cs.ShipRecoveryConflicts, ",")},
	})
}

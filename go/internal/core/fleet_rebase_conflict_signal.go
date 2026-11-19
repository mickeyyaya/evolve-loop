package core

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeFleetRebaseConflict signalcenter.Code = "ORCHESTRATOR_FLEET_REBASE_CONFLICT"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeFleetRebaseConflict,
		"the ship-recovery fleet rebase hit a merge conflict, and the cycle routes to debugger (ADR-0143 W1b reads it for optimistic admission). Not a landing-conflict error, so oversight does not double-count it; WARN. fields.paths (conflicted paths)")
}

func (o *Orchestrator) emitFleetRebaseConflict(cycle int, cs CycleState) {
	o.signals.Emit(signalcenter.Event{
		Cycle: cycle, RunID: cs.RunID, Phase: string(PhaseShip), Module: signalcenter.ModuleOrchestrator, Origin: "Orchestrator.recoverFromShipError",
		Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityWarn, Code: CodeFleetRebaseConflict,
		Reason: "fleet rebase conflict", Fields: map[string]string{"paths": strings.Join(cs.ShipRecoveryConflicts, ",")},
	})
}

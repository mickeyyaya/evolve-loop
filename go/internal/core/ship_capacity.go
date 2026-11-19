package core

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeCapacityParked signalcenter.Code = "GATE_REPO_CAPACITY_PARKED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleGateRepo, CodeCapacityParked, "a repo-contract gate hit its deadline twice in a row on the same lane tree. The orchestrator parked the lane without an LLM re-audit; fields.code is the ship error code. The lane needs more CPU time or a smaller closure, not a code change")
}

func capacityRepeats(code ShipErrorCode, cs CycleState) bool {
	return code == shiperr.CodeRepoContractCapacity && cs.ShipRecoveryCode == string(shiperr.CodeRepoContractCapacity)
}

func (o *Orchestrator) parkedOnCapacity(cs CycleState, se *ShipError) bool {
	if !capacityRepeats(se.Code, cs) {
		return false
	}
	fmt.Fprintf(o.errOut(), "[orchestrator] cycle %d gate deadline killed the same lane tree twice (%s); lane parked with no re-audit\n", cs.CycleID, se.Code)
	o.signals.Emit(signalcenter.Event{
		Cycle: cs.CycleID, RunID: cs.RunID, Phase: string(PhaseShip),
		Module: signalcenter.ModuleGateRepo, Origin: "Orchestrator.parkOnCapacity", Kind: signalcenter.KindShipWarning,
		Severity: signalcenter.SeverityWarn, Code: CodeCapacityParked,
		Reason: fmt.Sprintf("cycle %d is parked: a gate hit its deadline twice on an unchanged lane tree and no LLM can fix a lack of CPU time; next: give the lane more CPU share or a smaller package closure, then start the item again", cs.CycleID),
		Fields: map[string]string{"code": string(se.Code)},
	})
	return true
}

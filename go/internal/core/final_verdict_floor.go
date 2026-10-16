package core

import "slices"

func (o *Orchestrator) isAuthoritativePhase(phase Phase) bool {
	if phase == PhaseShip {
		return true
	}
	return slices.Contains(o.resolvedShipFloor(), string(phase))
}

func (o *Orchestrator) floorAlreadyCompleted(completed []string) bool {
	for _, p := range completed {
		if o.isAuthoritativePhase(Phase(p)) {
			return true
		}
	}
	return false
}

func (o *Orchestrator) recordFinalVerdict(result *CycleResult, phase Phase, verdict string, floorAlreadyRecorded bool) {
	if o.isAuthoritativePhase(phase) || !floorAlreadyRecorded {
		result.FinalVerdict = verdict
		return
	}
	if verdict != VerdictPASS {
		result.VerdictsNotAdopted = append(result.VerdictsNotAdopted, VerdictNotAdopted{Phase: string(phase), Verdict: verdict})
	}
}

func (o *Orchestrator) nonFloorExhaustionDegrade(phase Phase, workspace string, floorAlreadyRecorded bool) (PhaseResponse, bool) {
	if o.isAuthoritativePhase(phase) || !floorAlreadyRecorded {
		return PhaseResponse{}, false
	}
	return skippedResponse(phase, workspace), true
}

func skippedResponse(phase Phase, workspace string) PhaseResponse {
	return PhaseResponse{Phase: string(phase), Verdict: VerdictSKIPPED, ArtifactsDir: workspace}
}

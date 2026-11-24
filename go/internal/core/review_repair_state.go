package core

import "github.com/mickeyyaya/evolve-loop/go/internal/codereview"

func reviewPhaseRecorded(phase Phase, verdict string) bool {
	return phase == Phase(codereview.PhaseName) && verdict != VerdictFAIL && verdict != VerdictSKIPPED
}

package core

import (
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

// resumeFinalVerdict recovers host disposition, never an agent report's prose.
// An old checkpoint without the last completed floor's evidence stays WARN
// until another authoritative phase runs; missing evidence cannot imply PASS.
func (o *Orchestrator) resumeFinalVerdict(cs CycleState) string {
	if IsVerdict(cs.FinalVerdict) {
		return cs.FinalVerdict
	}
	if len(cs.AuditFailReasons) > 0 || len(cs.ShipFailReasons) > 0 {
		return VerdictFAIL
	}
	for i := len(cs.CompletedPhases) - 1; i >= 0; i-- {
		phase := cs.CompletedPhases[i]
		if !o.isAuthoritativePhase(Phase(phase)) {
			continue
		}
		entries, err := phasetiming.Read(cs.WorkspacePath)
		if err == nil {
			for j := len(entries) - 1; j >= 0; j-- {
				entry := entries[j]
				if entry.Phase == phase && entry.AbortReason == "" && IsVerdict(entry.Verdict) {
					return entry.Verdict
				}
			}
		}
		fmt.Fprintf(os.Stderr, "[resume] WARN cycle %d: legacy checkpoint has no host verdict for completed %s; retaining WARN until an authoritative phase runs\n", cs.CycleID, phase)
		return VerdictWARN
	}
	return VerdictPASS // no authoritative phase has completed yet
}

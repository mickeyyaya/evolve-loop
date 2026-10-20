package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

type resumeWitness struct {
	Cycle        int    `json:"cycle"`
	RunID        string `json:"run_id"`
	Shipped      bool   `json:"shipped"`
	PreCycleHEAD string `json:"pre_cycle_head"`
}

func resumeWitnessPath(projectRoot string, cycle int) string {
	return filepath.Join(paths.EvolveDirOf(projectRoot), "resume-witness", fmt.Sprintf("cycle-%d.json", cycle))
}

func recordResumeWitness(projectRoot string, cs CycleState) error {
	return atomicwrite.JSON(resumeWitnessPath(projectRoot, cs.CycleID), resumeWitness{
		Cycle:        cs.CycleID,
		RunID:        cs.RunID,
		Shipped:      cs.Shipped,
		PreCycleHEAD: cs.PreCycleHEAD,
	})
}

func (o *Orchestrator) persistResumeWitness(projectRoot string, cs CycleState) {
	if err := recordResumeWitness(projectRoot, cs); err != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN cycle %d: resume witness not recorded (%v); a resume will treat the ship latch as unset and the pre-cycle HEAD as unknown\n", cs.CycleID, err)
	}
}

func (o *Orchestrator) latchShip(projectRoot string, cs *CycleState, phase Phase, verdict string) bool {
	if !latchShippedState(cs, phase, verdict) {
		return false
	}
	o.persistResumeWitness(projectRoot, *cs)
	return true
}

func applyResumeWitness(projectRoot string, cycle int, cs *CycleState) {
	cs.Shipped, cs.PreCycleHEAD = false, ""
	body, err := os.ReadFile(resumeWitnessPath(projectRoot, cycle))
	if err != nil {
		return
	}
	var w resumeWitness
	if json.Unmarshal(body, &w) != nil || w.Cycle != cycle || w.RunID != cs.RunID {
		return
	}
	cs.Shipped, cs.PreCycleHEAD = w.Shipped, w.PreCycleHEAD
}

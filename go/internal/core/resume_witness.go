package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

type witnessShipError struct {
	Code    string `json:"code"`
	Class   string `json:"class"`
	Message string `json:"message"`
}

type resumeWitness struct {
	Cycle        int               `json:"cycle"`
	RunID        string            `json:"run_id"`
	Shipped      bool              `json:"shipped"`
	PreCycleHEAD string            `json:"pre_cycle_head"`
	ShipError    *witnessShipError `json:"ship_error,omitempty"`
}

func resumeWitnessPath(projectRoot string, cycle int) string {
	return filepath.Join(paths.EvolveDirOf(projectRoot), "resume-witness", fmt.Sprintf("cycle-%d.json", cycle))
}

func readResumeWitness(projectRoot string, cycle int) (resumeWitness, bool) {
	body, err := os.ReadFile(resumeWitnessPath(projectRoot, cycle))
	if err != nil {
		return resumeWitness{}, false
	}
	var w resumeWitness
	if json.Unmarshal(body, &w) != nil || w.Cycle != cycle {
		return resumeWitness{}, false
	}
	return w, true
}

func recordResumeWitness(projectRoot string, cs CycleState) error {
	return writeResumeWitness(projectRoot, cs, nil)
}

func writeResumeWitness(projectRoot string, cs CycleState, shipErr *witnessShipError) error {
	w := resumeWitness{Cycle: cs.CycleID, RunID: cs.RunID, Shipped: cs.Shipped, PreCycleHEAD: cs.PreCycleHEAD, ShipError: shipErr}
	if shipErr == nil {
		if prior, ok := readResumeWitness(projectRoot, cs.CycleID); ok && prior.RunID == cs.RunID {
			w.ShipError = prior.ShipError
		}
	}
	return atomicwrite.JSON(resumeWitnessPath(projectRoot, cs.CycleID), w)
}

func (o *Orchestrator) persistResumeWitness(projectRoot string, cs CycleState) {
	if err := recordResumeWitness(projectRoot, cs); err != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN cycle %d: resume witness not recorded (%v); a resume will treat the ship latch as unset and the pre-cycle HEAD as unknown\n", cs.CycleID, err)
	}
}

func (o *Orchestrator) persistShipErrorWitness(projectRoot string, cs CycleState, se *ShipError) {
	shipErr := &witnessShipError{Code: string(se.Code), Class: string(se.Class), Message: se.Message}
	if err := writeResumeWitness(projectRoot, cs, shipErr); err != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN cycle %d: ship error not recorded in the resume witness (%v); the lost-landing floor falls back to the workspace ship-error.json\n", cs.CycleID, err)
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
	w, ok := readResumeWitness(projectRoot, cycle)
	if !ok || w.RunID != cs.RunID {
		return
	}
	cs.Shipped, cs.PreCycleHEAD = w.Shipped, w.PreCycleHEAD
}

func hostShipError(projectRoot string, cycle int) (code, class, msg string, ok bool) {
	w, found := readResumeWitness(projectRoot, cycle)
	if !found || w.ShipError == nil || w.ShipError.Code == "" {
		return "", "", "", false
	}
	return w.ShipError.Code, w.ShipError.Class, w.ShipError.Message, true
}

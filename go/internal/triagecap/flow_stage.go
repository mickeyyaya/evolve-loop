package triagecap

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

var errNoDeclaredFloors = errors.New("it declares no committed_floors")

func declaredDecisionError(companionPath string) error {
	raw, err := os.ReadFile(companionPath)
	if err != nil {
		return err
	}
	if err := phasecontract.MustFlowDecision(phasecontract.FlowSeatTriage).Validate(raw); err != nil {
		return err
	}
	var decision struct {
		Floors *[]string `json:"committed_floors"`
	}
	if err := json.Unmarshal(raw, &decision); err != nil {
		return err
	}
	if decision.Floors == nil {
		return errNoDeclaredFloors
	}
	return nil
}

func (r *CapReviewer) refuseProseFloors(in core.ReviewInput, companionPath string, stage phasecontract.FlowStage) (core.ReviewResult, bool) {
	if stage != phasecontract.FlowStageEnforce {
		return core.ReviewResult{}, false
	}
	err := declaredDecisionError(companionPath)
	if err == nil {
		return core.ReviewResult{}, false
	}
	reason := "the I-8 stage enforce refused the floor count of the triage report prose: " + filepath.Base(companionPath) + " " + err.Error() +
		"; write " + TriageDecisionName() + " with \"top_n\" and \"committed_floors\":[...] (an empty list when no floor is committed)"
	r.emitProseViolation(in, phasecontract.FlowViolation{Stage: stage, Action: "refused", Reason: reason})
	if r.stage != config.StageEnforce {
		r.logf("[triage-cap] %s (stage=%s, would-block)", reason, r.stage)
		return core.ReviewResult{Approve: true}, true
	}
	return core.ReviewResult{Approve: false, Reason: reason}, true
}

func (r *CapReviewer) reportProseFloors(in core.ReviewInput, companionPath string, stage phasecontract.FlowStage) {
	r.emitProseViolation(in, phasecontract.FlowViolation{Stage: stage, Action: "logged",
		Reason: "the triage capacity clamp counted the floors in the triage report prose because " + filepath.Base(companionPath) + " declares no valid committed_floors"})
}

func (r *CapReviewer) emitProseViolation(in core.ReviewInput, v phasecontract.FlowViolation) {
	v.Seat, v.Rule = phasecontract.FlowSeatTriage, phasecontract.FlowRuleProseFallback
	cell := phasecontract.FlowCell{Cycle: in.Cycle, Phase: in.Phase, Origin: "CapReviewer.Review", Path: filepath.Join(in.Workspace, TriageArtifactName())}
	in.Signals.Emit(phasecontract.FlowViolationEvent(v, cell))
}

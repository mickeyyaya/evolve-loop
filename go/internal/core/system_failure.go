package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type floorFailReason struct {
	SchemaVersion int      `json:"schema_version"`
	Phase         string   `json:"phase"`
	Reasons       []string `json:"reasons"`
}

func floorFailReasonPath(workspace string, phase Phase) string {
	return filepath.Join(workspace, string(phase)+"-fail-reason.json")
}

func persistFloorFailReasons(cs *CycleState, phase Phase, diags []Diagnostic) {
	if cs == nil {
		return
	}
	reasons := cyclestate.ErrorMessages(diags)
	if phase == PhaseAudit {
		cs.AuditFailReasons = reasons
	}
	if cs.WorkspacePath == "" {
		return
	}
	path := floorFailReasonPath(cs.WorkspacePath, phase)
	if len(reasons) == 0 {
		_ = os.Remove(path)
		return
	}
	b, err := json.MarshalIndent(floorFailReason{SchemaVersion: 1, Phase: string(phase), Reasons: reasons}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o644)
}

func runnerDiagnosedAudit(cs CycleState) bool {
	return len(cs.AuditFailReasons) > 0
}

func resetFloorFailReason(cs *CycleState, phase Phase) {
	if cs == nil {
		return
	}
	if phase == PhaseAudit {
		cs.AuditFailReasons = nil
	}
	if phase == PhaseShip {
		cs.ShipFailReasons = nil
	}
	if cs.WorkspacePath == "" {
		return
	}
	if phase == PhaseAudit {
		recordedPhase, _ := readAuditFailReason(cs.WorkspacePath)
		if recordedPhase != "" && recordedPhase != string(phase) {
			return
		}
	}
	_ = os.Remove(floorFailReasonPath(cs.WorkspacePath, phase))
}

func readFloorFailReasons(workspace string, phase Phase) []string {
	b, err := os.ReadFile(floorFailReasonPath(workspace, phase))
	if err != nil {
		return nil
	}
	var r floorFailReason
	if json.Unmarshal(b, &r) != nil {
		return nil
	}
	return r.Reasons
}

func (o *Orchestrator) detectVerdictIncoherence(ctx context.Context, cs CycleState, finalVerdict string) (sig *SystemFailureSignal, reconciled bool) {
	audit, acs, auditRan := coherence.ReadCycleVerdicts(cs.WorkspacePath)
	deliverableValid := false
	if auditRan && o.contractVerifier != nil {
		in := ReviewInput{Phase: string(PhaseAudit), Workspace: cs.WorkspacePath, Worktree: cs.ActiveWorktree}
		if res, err := o.contractVerifier.VerifyDeliverable(ctx, in); err == nil {
			deliverableValid = res.OK
		}
	}
	coh := coherence.CheckVerdictCoherence(coherence.VerdictInputs{
		Recorded:         finalVerdict,
		Audit:            audit,
		ACS:              acs,
		AuditRan:         auditRan,
		SubstantiveError: hasSubstantiveFailReasons(cs),
		DeliverableValid: deliverableValid,
	})
	if coh.Reconciled {
		return nil, true
	}
	if !coh.Incoherent || !o.failurePolicy.IsFloor(coh.Category) {
		return nil, false
	}
	return &SystemFailureSignal{
		Category: coh.Category,
		Level:    policy.LevelSystem,
		Evidence: coh.Evidence,
		Halt:     true,
	}, false
}

func hasSubstantiveFailReasons(cs CycleState) bool {
	return len(cs.AuditFailReasons) > 0 || len(cs.ShipFailReasons) > 0
}

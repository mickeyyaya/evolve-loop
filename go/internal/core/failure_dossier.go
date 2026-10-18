package core

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type auditDeclared struct {
	Class   string   `json:"class,omitempty"`
	Level   string   `json:"level,omitempty"`
	Defects []string `json:"defects,omitempty"`
}

type nonProgress struct {
	RepeatCeiling            int `json:"repeat_ceiling"`
	VerifiedNotLandedCeiling int `json:"verified_not_landed_ceiling"`
	TaskRetryCeiling         int `json:"task_retry_ceiling"`
	SameClassStreak          int `json:"same_class_streak"`
	RepeatCount              int `json:"repeat_count"`
}

type failureDossier struct {
	CycleID         int           `json:"cycle_id"`
	RecordedVerdict string        `json:"recorded_verdict"`
	FloorCandidate  string        `json:"floor_candidate,omitempty"`
	AuditDeclared   auditDeclared `json:"audit_declared"`
	NonProgress     nonProgress   `json:"non_progress"`
	Evidence        string        `json:"evidence,omitempty"`
}

func buildFailureDossier(cs CycleState, finalVerdict string, fp policy.SystemFailurePolicy) *failureDossier {
	d := &failureDossier{
		CycleID:         cs.CycleID,
		RecordedVerdict: finalVerdict,
		NonProgress: nonProgress{
			RepeatCeiling:            fp.Thresholds.RepeatCeiling,
			VerifiedNotLandedCeiling: fp.Thresholds.VerifiedNotLandedCeiling,
			TaskRetryCeiling:         fp.Thresholds.TaskRetryCeiling,
			SameClassStreak:          sameClassStreak(cs.FailedAt),
			RepeatCount:              len(cs.FailedAt),
		},
	}

	audit, acs, auditRan := coherence.ReadCycleVerdicts(cs.WorkspacePath)
	coh := coherence.CheckVerdictCoherence(coherence.VerdictInputs{
		Recorded:         finalVerdict,
		Audit:            audit,
		ACS:              acs,
		AuditRan:         auditRan,
		SubstantiveError: hasSubstantiveFailReasons(cs),
	})

	if fb, ok := phasecontract.ReadFailureBlock(cs.WorkspacePath, string(PhaseAudit)); ok {
		d.AuditDeclared.Class = fb.Class
		d.AuditDeclared.Defects = fb.Defects
		if cat, ok := fp.Categories[fb.Class]; ok {
			d.AuditDeclared.Level = cat.Level
		}
	}

	switch {
	case coh.Incoherent && fp.IsFloor(coh.Category):
		d.FloorCandidate = coh.Category
		d.Evidence = coh.Evidence
	case d.AuditDeclared.Level == policy.LevelSystem && fp.IsFloor(d.AuditDeclared.Class):
		d.FloorCandidate = d.AuditDeclared.Class
		d.Evidence = "audit self-declared a structured system-level class (" + d.AuditDeclared.Class + "): " +
			strings.Join(d.AuditDeclared.Defects, " | ")
	}
	return d
}

func sameClassStreak(history []FailedRecord) int {
	if len(history) == 0 {
		return 0
	}
	last := history[len(history)-1].Classification
	if last == "" {
		return 0
	}
	n := 0
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Classification != last {
			break
		}
		n++
	}
	return n
}

func writeFailureDossier(workspace string, d *failureDossier) error {
	if workspace == "" || d == nil {
		return nil
	}
	b, err := marshalIndentJSON(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(workspace, "failure-dossier.json"), b, 0o644)
}

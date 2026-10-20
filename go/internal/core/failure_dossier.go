package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/auditledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const failureDossierFile = "failure-dossier.json"

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
	CycleID         int            `json:"cycle_id"`
	RecordedVerdict string         `json:"recorded_verdict"`
	FloorCandidate  string         `json:"floor_candidate,omitempty"`
	AuditDeclared   auditDeclared  `json:"audit_declared"`
	BuildDeclared   *auditDeclared `json:"build_declared,omitempty"`
	NonProgress     nonProgress    `json:"non_progress"`
	Evidence        string         `json:"evidence,omitempty"`
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

func (o *Orchestrator) failureDossier(cs CycleState, finalVerdict string) *failureDossier {
	d := buildFailureDossier(cs, finalVerdict, o.failurePolicy)
	if d.AuditDeclared.Class != "" {
		return d
	}
	if bd := builderSystemClassUnderGates(cs, o.failurePolicy); bd != nil && o.auditedTreeUnchanged(cs.RunID) {
		d.BuildDeclared = bd
	}
	return d
}

func (o *Orchestrator) auditedTreeUnchanged(runID string) bool {
	if o.ledger == nil {
		return false
	}
	it, err := o.ledger.Iter(context.Background())
	if err != nil {
		return false
	}
	defer func() { _ = it.Close() }()
	var trees []string
	for {
		e, ok, err := it.Next()
		if err != nil || !ok {
			break
		}
		if auditledger.IsAuditorRow(auditledger.Entry{Role: e.Role, Kind: e.Kind}) && e.RunID == runID {
			trees = append(trees, e.WorktreeTreeSHA)
		}
	}
	n := len(trees)
	return n >= 2 && trees[n-1] != "" && trees[n-1] == trees[n-2]
}

func builderSystemClassUnderGates(cs CycleState, fp policy.SystemFailurePolicy) *auditDeclared {
	if _, gateForced := gateForcedDiagnoses(cs.AuditFailReasons); !gateForced || cs.AuditRepairAttempts == 0 {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(cs.WorkspacePath, "build-report.md"))
	if err != nil {
		return nil
	}
	s, ok := phasecontract.ParseVerdictSentinelFull(string(raw))
	if !ok || s.Failure == nil || (s.Verdict != VerdictWARN && s.Verdict != VerdictFAIL) {
		return nil
	}
	fb := s.Failure
	category := policyCategoryFor(failurelog.NormalizeLegacy(fb.Class))
	if cat, known := fp.Categories[category]; !known || cat.Level != policy.LevelSystem {
		return nil
	}
	return &auditDeclared{Class: category, Level: policy.LevelSystem, Defects: fb.Defects}
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
	return atomicwrite.Bytes(filepath.Join(workspace, failureDossierFile), b)
}

func (o *Orchestrator) writeDossier(cs CycleState, d *failureDossier) {
	if err := writeFailureDossier(cs.WorkspacePath, d); err != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN cycle %d: %s not written (%v); no decision reads it\n", cs.CycleID, failureDossierFile, err)
	}
}

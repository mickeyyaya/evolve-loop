package core

// See ADR-0072.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// auditDeclared is the audit phase's self-declared failure envelope, surfaced
// verbatim so the orchestrator judgment layer can classify a system-class
// fault the deterministic floor cannot catch.
//
// See ADR-0039.
type auditDeclared struct {
	Class   string   `json:"class,omitempty"`
	Level   string   `json:"level,omitempty"`
	Defects []string `json:"defects,omitempty"`
}

// nonProgress carries the policy thresholds (deterministic) alongside the
// counters computed from cycle history — the raw material for the non-progress
// system-level category.
type nonProgress struct {
	RepeatCeiling            int `json:"repeat_ceiling"`
	VerifiedNotLandedCeiling int `json:"verified_not_landed_ceiling"`
	TaskRetryCeiling         int `json:"task_retry_ceiling"`
	SameClassStreak          int `json:"same_class_streak"`
	RepeatCount              int `json:"repeat_count"`
}

// failureDossier is the independent-evidence packet written to
// <workspace>/failure-dossier.json.
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

	// The dossier builder has no ContractVerifier, so DeliverableValid stays
	// false: reconcile is the live floor's job, not the dossier's.
	audit, acs, auditRan := coherence.ReadCycleVerdicts(cs.WorkspacePath)
	coh := coherence.CheckVerdictCoherence(coherence.VerdictInputs{
		Recorded:         finalVerdict,
		Audit:            audit,
		ACS:              acs,
		AuditRan:         auditRan,
		SubstantiveError: hasSubstantiveFailReasons(cs),
		FailReasons:      cs.AuditFailReasons,
	})

	if fb, ok := phasecontract.ReadFailureBlock(cs.WorkspacePath, string(PhaseAudit)); ok {
		d.AuditDeclared.Class = fb.Class
		d.AuditDeclared.Defects = fb.Defects
		if cat, ok := fp.Categories[fb.Class]; ok {
			d.AuditDeclared.Level = cat.Level
		}
	}

	// Floor candidate: a broken pipeline cannot be talked out of these.
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

// sameClassStreak counts the trailing run of history records sharing the
// most-recent failure classification — the same-class recurrence counter the
// non-progress category keys off. Empty history or a blank latest class → 0.
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

// writeFailureDossier persists the dossier to <workspace>/failure-dossier.json.
// Best-effort at the call site (forensics must never abort a cycle); a blank
// workspace or nil dossier is a no-op.
func writeFailureDossier(workspace string, d *failureDossier) error {
	if workspace == "" || d == nil {
		return nil
	}
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(workspace, "failure-dossier.json"), b, 0o644)
}

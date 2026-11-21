package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxfile"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeAuditRereviewShadow     signalcenter.Code = "AUDIT_REREVIEW_SHADOW"
	CodeAuditRereviewNoScope    signalcenter.Code = "AUDIT_REREVIEW_NO_SCOPE"
	CodeAuditRereviewFileFailed signalcenter.Code = "AUDIT_REREVIEW_FILE_FAILED"

	AuditLateFindingsFile = "audit-late-findings.json"

	lateFindingIDPrefix = "audit-late-"
	lateFindingWeight   = 0.3
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeAuditRereviewShadow, "from audit round 2, the host measured the repair-diff re-review (ADR-0168). fields.stage is shadow (prompt and verdict unchanged) or enforce. fields.round, hunks, late, would_file (late, below the late-file bar of the audit-repair loop), would_block (late, at or above that bar), reworded (new ids at the location of an open previous finding), filed (follow-ups made at enforce) and verdict")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeAuditRereviewFileFailed, "at enforce, a late audit finding was not filed as a follow-up, or the landing note was not written. fields.round, fields.reason names the failure")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeAuditRereviewNoScope, "an audit round from 2 on has no repair-diff scope, so it ran as a full audit and nothing was filed (ADR-0168 D3). fields.round, fields.reason names the missing input")
}

func (o *Orchestrator) auditRereviewStage() policy.AuditRereviewStage {
	return o.workflowConfig.Convergence.AuditRereview
}

func (o *Orchestrator) auditScopeFor(ctx context.Context, phase Phase, cs CycleState, projectRoot string) *AuditRoundScope {
	if phase != PhaseAudit || cs.AuditDispatches < 2 {
		return nil
	}
	scope, reason := computeAuditScope(ctx, scopeInputs{
		ledger: o.ledger, cycle: cs.CycleID, round: cs.AuditDispatches,
		projectRoot: projectRoot, worktree: cs.ActiveWorktree, workspace: cs.WorkspacePath,
	})
	if reason != "" {
		o.emitAuditRereviewNoScope(cs, reason)
		return nil
	}
	scope.Enforce = o.auditRereviewStage() == policy.AuditRereviewEnforce
	if err := writeAuditScope(cs.WorkspacePath, scope); err != nil {
		o.emitAuditRereviewNoScope(cs, "the scope file cannot be written: "+err.Error())
		return nil
	}
	return &scope
}

func (o *Orchestrator) emitAuditRereviewNoScope(cs CycleState, reason string) {
	o.signals.Emit(signalcenter.Event{
		Cycle: cs.CycleID, RunID: o.signalRunID(), Phase: string(PhaseAudit), Attempt: cs.AuditDispatches,
		Module: signalcenter.ModuleAudit, Origin: "Orchestrator.auditScopeFor",
		Kind: signalcenter.KindPhaseOutcome, Severity: signalcenter.SeverityInfo, Code: CodeAuditRereviewNoScope,
		Reason: "audit round " + strconv.Itoa(cs.AuditDispatches) + " runs as a full audit: " + reason,
		Fields: map[string]string{"round": strconv.Itoa(cs.AuditDispatches), "reason": reason},
	})
}

type rereviewCounts struct {
	late, wouldFile, wouldBlock, reworded int
}

func countRereview(scope *AuditRoundScope, findings []reportdoc.Finding) rereviewCounts {
	var c rereviewCounts
	for _, f := range findings {
		switch {
		case scope.Late(f):
			c.late++
			if convergence.LateFiled(convergence.LoopAuditRepair, lateMark(f)) {
				c.wouldFile++
			} else {
				c.wouldBlock++
			}
		case scope.reworded(f):
			c.reworded++
		}
	}
	return c
}

func (o *Orchestrator) recordAuditRereview(ctx context.Context, cs CycleState, verdict string) {
	if cs.AuditDispatches < 2 {
		return
	}
	scope := readAuditScope(cs.WorkspacePath, cs.AuditDispatches)
	if scope == nil {
		return
	}
	raw, err := os.ReadFile(filepath.Join(cs.WorkspacePath, auditReportName))
	if err != nil {
		return
	}
	report := string(raw)
	filed := o.fileLateFindings(cs, scope, reportdoc.FiledLate(report))
	o.emitAuditRereviewShadow(cs, scope, countRereview(scope, reportdoc.Findings(report)), len(filed), verdict)
}

func (o *Orchestrator) emitAuditRereviewShadow(cs CycleState, scope *AuditRoundScope, c rereviewCounts, filed int, verdict string) {
	stage := string(o.auditRereviewStage())
	o.signals.Emit(signalcenter.Event{
		Cycle: cs.CycleID, RunID: o.signalRunID(), Phase: string(PhaseAudit), Attempt: cs.AuditDispatches,
		Module: signalcenter.ModuleAudit, Origin: "Orchestrator.recordAuditRereview",
		Kind: signalcenter.KindPhaseOutcome, Severity: signalcenter.SeverityInfo, Code: CodeAuditRereviewShadow,
		Reason: fmt.Sprintf("%s: %d late finding(s), %d would be filed, %d would block", stage, c.late, c.wouldFile, c.wouldBlock),
		Fields: map[string]string{
			"stage": stage, "round": strconv.Itoa(cs.AuditDispatches), "hunks": strconv.Itoa(len(scope.Hunks)),
			"late": strconv.Itoa(c.late), "would_file": strconv.Itoa(c.wouldFile), "would_block": strconv.Itoa(c.wouldBlock),
			"reworded": strconv.Itoa(c.reworded), "filed": strconv.Itoa(filed), "verdict": verdict,
		},
	})
}

func (s *AuditRoundScope) reworded(f reportdoc.Finding) bool {
	return f.Location != "" && !slices.Contains(s.KnownIDs, HostFindingID(f)) &&
		slices.ContainsFunc(s.Previous, func(p ScopeFinding) bool { return p.Location == f.Location })
}

func (o *Orchestrator) fileLateFindings(cs CycleState, scope *AuditRoundScope, entries []reportdoc.Finding) []reportdoc.Finding {
	if o.auditRereviewStage() != policy.AuditRereviewEnforce || len(entries) == 0 {
		return nil
	}
	var valid []reportdoc.Finding
	for _, e := range entries {
		if len(scope.CheckFiled([]reportdoc.Finding{e})) == 0 {
			valid = append(valid, e)
		}
	}
	inbox := inboxDirOf(cs)
	var filed []reportdoc.Finding
	var failed []string
	for _, e := range valid {
		if err := mintLateFinding(inbox, cs, e, o.now); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", e.ID, err))
			continue
		}
		filed = append(filed, e)
	}
	o.recordLateFindings(cs, valid, failed)
	return filed
}

func (o *Orchestrator) recordLateFindings(cs CycleState, late []reportdoc.Finding, failed []string) {
	doc := struct {
		Cycle  int                 `json:"cycle"`
		Round  int                 `json:"round"`
		Late   []reportdoc.Finding `json:"late"`
		Failed []string            `json:"failed,omitempty"`
	}{cs.CycleID, cs.AuditDispatches, late, failed}
	if err := atomicwrite.JSON(filepath.Join(cs.WorkspacePath, AuditLateFindingsFile), doc); err != nil {
		failed = append(failed, "the landing note is not written: "+err.Error())
	}
	if len(failed) == 0 {
		return
	}
	o.signals.Emit(signalcenter.Event{
		Cycle: cs.CycleID, RunID: o.signalRunID(), Phase: string(PhaseAudit), Attempt: cs.AuditDispatches,
		Module: signalcenter.ModuleAudit, Origin: "Orchestrator.fileLateFindings",
		Kind: signalcenter.KindPhaseOutcome, Severity: signalcenter.SeverityWarn, Code: CodeAuditRereviewFileFailed,
		Reason: "a late audit finding was not filed: " + strings.Join(failed, "; "),
		Fields: map[string]string{"round": strconv.Itoa(cs.AuditDispatches), "reason": strings.Join(failed, "; ")},
	})
}

type lateFindingItem struct {
	ID            string   `json:"id"`
	Weight        float64  `json:"weight"`
	Kind          string   `json:"kind"`
	Priority      string   `json:"priority"`
	PriorityClass string   `json:"priority_class"`
	Origin        string   `json:"origin"`
	Title         string   `json:"title"`
	Summary       string   `json:"summary"`
	Fix           string   `json:"fix"`
	Files         []string `json:"files,omitempty"`
	Acceptance    []string `json:"acceptance"`
	Source        string   `json:"source"`
}

func mintLateFinding(inboxDir string, cs CycleState, f reportdoc.Finding, now func() time.Time) error {
	file, _, _ := strings.Cut(f.Location, ":")
	title := findingTitleKey(f)
	item := lateFindingItem{
		ID:            lateFindingIDPrefix + strings.TrimPrefix(HostFindingID(f), "d")[:16],
		Weight:        lateFindingWeight,
		Kind:          "bug",
		Priority:      strings.ToLower(f.Severity),
		PriorityClass: inboxbatch.ClassCorrectness,
		Origin:        inboxbatch.OriginCycle,
		Title:         inboxbatch.Fit("Late audit finding: "+title, inboxbatch.MaxFieldLen),
		Summary:       fmt.Sprintf("Audit round %d of cycle %d found a %s defect at %s in code that no repair of the cycle changed. The re-review filed it and did not block the cycle (ADR-0168).", cs.AuditDispatches, cs.CycleID, f.Severity, f.Location),
		Fix:           "Reproduce the defect at the named location, fix it at its source, and add a regression test that fails before the fix.",
		Files:         []string{file},
		Acceptance:    []string{inboxbatch.Fit("The defect at "+f.Location+" is fixed", inboxbatch.MaxAcceptanceLen), "A regression test fails before the fix and passes after it"},
		Source:        "audit round " + strconv.Itoa(cs.AuditDispatches) + " of cycle " + strconv.Itoa(cs.CycleID),
	}
	raw, _ := json.Marshal(item)
	_, err := inboxfile.Mint(inboxDir, raw, now)
	return err
}

package core

import (
	"cmp"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeConvergenceRung signalcenter.Code = "CONVERGENCE_RUNG"

	auditReportName          = "audit-report.md"
	convergenceEnforceStage  = "enforce"
	auditFindingClass        = "correctness"
	convergenceStopReasonFmt = "convergence decided Stop at rung %d"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleConvergence, CodeConvergenceRung, "the convergence policy decided the next rung of the audit-repair loop after an audit FAIL; fields.stage is shadow (recorded, the repair flow does not change) or enforce (a Stop ends the repair), fields.loop, fields.round, fields.action, fields.rung and fields.blocking_bar give the decision, fields.fresh_context is true when the next fixer starts without the failed context")
}

func auditRepairJudgments(workspace string, dispatches int, warn io.Writer) []convergence.Judgment {
	var reports []string
	for round := 1; round < dispatches; round++ {
		if raw, err := os.ReadFile(filepath.Join(workspace, phasecontract.RoundArchiveFilename(auditReportName, round))); err == nil {
			reports = append(reports, string(raw))
		}
	}
	live, err := os.ReadFile(filepath.Join(workspace, auditReportName))
	if err != nil {
		fmt.Fprintf(warn, "audit-repair convergence: no live %s, no decision: %v\n", auditReportName, err)
		return nil
	}
	return judgmentsOf(append(reports, string(live)))
}

func judgmentsOf(reports []string) []convergence.Judgment {
	judgments := make([]convergence.Judgment, 0, len(reports))
	var previous map[string]convergence.Finding
	for i, report := range reports {
		open := openAuditFindings(report)
		judgments = append(judgments, convergence.Judgment{
			Index:    i,
			Findings: slices.Concat(sortedFindings(open), fixedSince(previous, open)),
		})
		previous = open
	}
	return judgments
}

func openAuditFindings(report string) map[string]convergence.Finding {
	open := map[string]convergence.Finding{}
	for _, f := range reportdoc.Findings(report) {
		id := cmp.Or(f.Title, f.ID, "finding")
		open[id] = convergence.Finding{ID: id, Severity: convergence.Severity(f.Severity), Class: auditFindingClass, Status: convergence.StatusOpen}
	}
	return open
}

func sortedFindings(byID map[string]convergence.Finding) []convergence.Finding {
	var out []convergence.Finding
	for _, id := range slices.Sorted(maps.Keys(byID)) {
		out = append(out, byID[id])
	}
	return out
}

func fixedSince(previous, open map[string]convergence.Finding) []convergence.Finding {
	gone := map[string]convergence.Finding{}
	for id, f := range previous {
		if _, still := open[id]; !still {
			f.Status = convergence.StatusFixed
			gone[id] = f
		}
	}
	return sortedFindings(gone)
}

func auditRepairConvergenceInput(cfg policy.ConvergenceConfig, rounds []convergence.Judgment, fixer, judge convergence.TierEffort, headroom convergence.HeadroomTable) convergence.Input {
	return convergence.Input{
		Loop: convergence.LoopAuditRepair, Round: len(rounds) - 1, Rounds: rounds,
		Fixer: fixer, Judge: judge, Headroom: headroom, Config: cfg,
	}
}

func (o *Orchestrator) consultAuditRepairConvergence(cs CycleState) string {
	cfg := o.workflowConfig.Convergence
	if cfg.Stage == "" {
		return ""
	}
	rounds := auditRepairJudgments(cs.WorkspacePath, cs.AuditDispatches, o.errOut())
	if len(rounds) == 0 {
		return ""
	}
	d := convergence.Decide(auditRepairConvergenceInput(cfg, rounds, convergence.TierEffort{}, convergence.TierEffort{}, nil))
	o.emitConvergenceRung(cs, cfg.Stage, len(rounds)-1, d)
	if cfg.Stage != convergenceEnforceStage || d.Action != convergence.ActionStop {
		return ""
	}
	return fmt.Sprintf(convergenceStopReasonFmt, d.Rung)
}

func (o *Orchestrator) emitConvergenceRung(cs CycleState, stage string, round int, d convergence.Decision) {
	o.signals.Emit(signalcenter.Event{
		Cycle: cs.CycleID, RunID: o.signalRunID(), Phase: string(PhaseAudit), Attempt: cs.AuditDispatches,
		Module: signalcenter.ModuleConvergence, Origin: "Orchestrator.consultAuditRepairConvergence",
		Kind: signalcenter.KindPhaseOutcome, Severity: signalcenter.SeverityInfo, Code: CodeConvergenceRung,
		Reason: strings.Join(d.Reasons, "; "),
		Fields: map[string]string{
			"stage": stage, "loop": string(convergence.LoopAuditRepair), "round": strconv.Itoa(round),
			"action": string(d.Action), "rung": strconv.Itoa(d.Rung), "blocking_bar": string(d.BlockingBar),
			"fresh_context": strconv.FormatBool(d.FreshContext),
		},
	})
}

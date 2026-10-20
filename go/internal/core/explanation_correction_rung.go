package core

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeExplanationCorrectionRung      signalcenter.Code = "ORCHESTRATOR_EXPLANATION_CORRECTION_RUNG"
	CodeExplanationCorrectionExhausted signalcenter.Code = "ORCHESTRATOR_EXPLANATION_CORRECTION_EXHAUSTED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeExplanationCorrectionRung, "a passing audit reviewed the explanation document as NEEDS_CORRECTION, so Build corrects only that document and the audit re-reviews it before ship; fields.round and fields.budget count the rung")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeExplanationCorrectionExhausted, "a passing audit still reviews the explanation document as NEEDS_CORRECTION after the explanation-correction budget is spent, so the verified code ships with the review on record")
}

type explanationReview struct {
	document string
	evidence string
}

func pendingExplanationCorrection(cs CycleState) (explanationReview, bool) {
	report, err := os.ReadFile(filepath.Join(cs.WorkspacePath, phasecontract.ArtifactFilename(string(PhaseAudit))))
	if err != nil {
		return explanationReview{}, false
	}
	fields, _, _ := reportdoc.ReasonedReview(string(report), "audit-report.md", phasecontract.ExplanationDocumentation.Title(), "Status", "Build status", "Document", "Document SHA256", "Evidence")
	if fields["status"] != "NEEDS_CORRECTION" {
		return explanationReview{}, false
	}
	document, err := explanationdocs.DocumentPath(cs.CycleID, cs.RunID)
	if err != nil {
		return explanationReview{}, false
	}
	return explanationReview{document: document, evidence: strings.TrimSpace(fields["evidence"])}, true
}

func (o *Orchestrator) explanationCorrectionBudget() int {
	category, _ := o.failurePolicy.RetryPolicyFor(policy.CategoryExplanationCorrection)
	return category.MaxRetries
}

func (o *Orchestrator) decideExplanationCorrection(cs *CycleState, phase Phase, verdict string) (Phase, bool) {
	if phase == PhaseBuild {
		cs.ExplanationCorrectionActive = false
		return "", false
	}
	if phase != PhaseAudit || (verdict != VerdictPASS && verdict != VerdictWARN) {
		return "", false
	}
	review, ok := pendingExplanationCorrection(*cs)
	if !ok {
		return "", false
	}
	budget := o.explanationCorrectionBudget()
	if len(cs.ExplanationCorrections) >= budget {
		o.emitExplanationCorrection(*cs, CodeExplanationCorrectionExhausted, signalcenter.SeverityWarn, budget,
			"explanation document still NEEDS_CORRECTION with the correction budget spent; the verified code ships: "+review.evidence)
		return "", false
	}
	cs.ExplanationCorrections = append(cs.ExplanationCorrections, review.evidence)
	cs.ExplanationCorrectionActive = true
	o.emitExplanationCorrection(*cs, CodeExplanationCorrectionRung, signalcenter.SeverityInfo, budget,
		"explanation document NEEDS_CORRECTION on a passing audit: Build corrects "+review.document+" alone, then the audit re-reviews it")
	return PhaseBuild, true
}

func (o *Orchestrator) emitExplanationCorrection(cs CycleState, code signalcenter.Code, severity signalcenter.Severity, budget int, reason string) {
	o.signals.Emit(signalcenter.Event{
		Cycle: cs.CycleID, RunID: o.signalRunID(), Phase: string(PhaseAudit), Attempt: cs.AuditDispatches,
		Module: signalcenter.ModuleOrchestrator, Origin: "Orchestrator.decideExplanationCorrection",
		Kind: signalcenter.KindPhaseOutcome, Severity: severity, Code: code, Reason: reason,
		Fields: map[string]string{"round": strconv.Itoa(len(cs.ExplanationCorrections)), "budget": strconv.Itoa(budget)},
	})
}

func explanationCorrectionBrief(document, evidence string) string {
	var b strings.Builder
	lines := regexp.MustCompile(regexp.QuoteMeta(document)+`(?::\d+(?:[-,]\d+)*|#L\d+(?:-L?\d+)?)`).FindAllString(evidence, -1)
	if len(lines) > 0 {
		b.WriteString("Document lines the review names:\n")
		for _, line := range lines {
			b.WriteString("- " + line + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("Review evidence: " + evidence)
	return b.String()
}

func seedExplanationCorrection(base map[string]string, cs CycleState) map[string]string {
	document, err := explanationdocs.DocumentPath(cs.CycleID, cs.RunID)
	if err != nil || len(cs.ExplanationCorrections) == 0 {
		return base
	}
	out := withContext(base, CtxKeyExplanationReauthor, document)
	return withContext(out, CtxKeyExplanationCorrection, explanationCorrectionBrief(document, cs.ExplanationCorrections[len(cs.ExplanationCorrections)-1]))
}

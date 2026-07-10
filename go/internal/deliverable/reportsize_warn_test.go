package deliverable

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// reportsize_warn_test.go — cycle-646 slice of report-handoff-size-contract-scout.
//
// cycle-565 Slice S1 shipped CheckHandoffBudget/VerifyWithReportSize/the
// Reviewer wiring, but left "shadow" and "advisory" BEHAVIORALLY IDENTICAL:
// VerifyWithReportSize returns immediately (no res.add) for any
// reportSizeGate < StageEnforce, so an oversized Handoff Summary produces
// exactly zero observable signal — not even the reviewer's own "would-block"
// log line — until the operator flips the gate straight to enforce. That is
// not the staged WARN-mode rollout the source spec
// (.evolve/inbox/processed/cycle-565/bb0f4815-...json) and this cycle's
// scout-report Task 2 describe ("checked by the contract gate in WARN mode
// (not enforce)").
//
// This slice narrows "advisory" into the missing WARN rung: it must RECORD
// the CodeHandoffBudgetExceeded violation (so the existing Reviewer.Review
// shadow/advisory branch logs "would-block" via r.logf) while still
// Approve==true (non-blocking). "shadow" stays fully silent/dormant exactly
// as TestVerifyWithReportSize_ShadowDoesNotViolate_EnforceDoes already pins —
// this file does not touch that contract.
//
// RED today: VerifyWithReportSize's early-return threshold is
// `reportSizeGate < config.StageEnforce`, so StageAdvisory is silent — no
// CodeHandoffBudgetExceeded violation is recorded and the Reviewer's
// "would-block" log line never fires for an oversized handoff section.

func TestVerifyWithReportSize_AdvisoryRecordsWarnViolation(t *testing.T) {
	ws := t.TempDir()
	big := strings.Repeat("word ", 5000)
	report := "## Changes\n- x\nVerdict: PASS\n## Handoff Summary\n" + big
	writeFile(t, ws, "build-report.md", report)
	roots := phasecontract.Roots{Workspace: ws}

	res, err := VerifyWithReportSize("build", roots, phasecontract.BuiltinResolver{}, config.StageOff, config.StageAdvisory, 2000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasCode(res, CodeHandoffBudgetExceeded) {
		t.Errorf("reportSizeGate=advisory (WARN mode) must record %s as an observed violation — so the reviewer's shadow/advisory branch logs a would-block warning instead of staying silent; got %+v", CodeHandoffBudgetExceeded, res.Violations)
	}
}

// TestVerifyWithReportSize_ShadowStaysSilent_Negative is the scope-boundary
// negative test: "shadow" is the fully-dormant rung (cycle-565 contract,
// pinned in TestVerifyWithReportSize_ShadowDoesNotViolate_EnforceDoes) and
// must NOT gain a violation from this slice — only "advisory" becomes WARN.
// A naive "advisory OR shadow both warn" implementation fails this.
func TestVerifyWithReportSize_ShadowStaysSilent_Negative(t *testing.T) {
	ws := t.TempDir()
	big := strings.Repeat("word ", 5000)
	report := "## Changes\n- x\nVerdict: PASS\n## Handoff Summary\n" + big
	writeFile(t, ws, "build-report.md", report)
	roots := phasecontract.Roots{Workspace: ws}

	res, err := VerifyWithReportSize("build", roots, phasecontract.BuiltinResolver{}, config.StageOff, config.StageShadow, 2000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasCode(res, CodeHandoffBudgetExceeded) {
		t.Errorf("reportSizeGate=shadow must remain fully silent (dormant) — the WARN rung is advisory-only; got %+v", res.Violations)
	}
}

// TestReviewer_ReportSizeGate_AdvisoryWarnsButApproves proves the end-to-end
// host-side effect: at reportSizeGate=advisory the Reviewer approves (never
// blocks) an oversized Handoff Summary, matching enforce's blocking
// counterpart already pinned by TestReviewer_ReportSizeGate_BlocksOnlyAtEnforce.
func TestReviewer_ReportSizeGate_AdvisoryWarnsButApproves(t *testing.T) {
	big := strings.Repeat("word ", 5000)
	report := "## Changes\n- x\nVerdict: PASS\n## Handoff Summary\n" + big
	ws := t.TempDir()
	writeFile(t, ws, "build-report.md", report)
	r := newTestReviewerReportSize(config.StageEnforce, config.StageAdvisory, 2000, filepath.Join(t.TempDir(), "b.json"), 3)
	got := r.Review(context.Background(), reviewInput("build", ws, t.TempDir()))
	if !got.Approve {
		t.Errorf("reportSizeGate=advisory (WARN) must never block; got Approve=false reason=%q", got.Reason)
	}
}

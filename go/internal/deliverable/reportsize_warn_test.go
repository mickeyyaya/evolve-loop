package deliverable

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

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

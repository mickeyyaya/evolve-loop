//go:build acs

package cycle646

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	deliverablePkg = "github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	promptsPkg     = "github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC646_001_ReportSizeGateAdvisoryWarns(t *testing.T) {
	ok, out := runGoTest(t, deliverablePkg, "TestVerifyWithReportSize_AdvisoryRecordsWarnViolation")
	if !ok {
		t.Errorf("advisory (WARN) rung not yet recording the handoff-budget violation:\n%s", out)
	}
}

func TestC646_002_ReportSizeGateShadowStaysSilent(t *testing.T) {
	ok, out := runGoTest(t, deliverablePkg, "TestVerifyWithReportSize_ShadowStaysSilent_Negative")
	if !ok {
		t.Errorf("shadow rung must stay silent (dormant) — WARN is advisory-only:\n%s", out)
	}
}

func TestC646_003_ReviewerAdvisoryWarnsButApproves(t *testing.T) {
	ok, out := runGoTest(t, deliverablePkg, "TestReviewer_ReportSizeGate_AdvisoryWarnsButApproves")
	if !ok {
		t.Errorf("reviewer must approve (non-blocking) at reportSizeGate=advisory:\n%s", out)
	}
}

func TestC646_004_PersonaCombinedLineCountReduced(t *testing.T) {
	ok, out := runGoTest(t, promptsPkg, "TestPersonaStopCriterionDedupe_CombinedLineCountReduced")
	if !ok {
		t.Errorf("combined evolve-scout/builder/auditor.md line count not yet reduced below the 751-line pre-dedupe baseline:\n%s", out)
	}
}

func TestC646_005_PersonaNoGateOrBannedPatternTextLost(t *testing.T) {
	ok, out := runGoTest(t, promptsPkg, "TestPersonaStopCriterionDedupe_NoGateOrBannedPatternTextLost")
	if !ok {
		t.Errorf("a gate name or banned-post-report phrase was lost from agents/evolve-*.md during dedupe:\n%s", out)
	}
}

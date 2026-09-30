//go:build acs

package cycle565

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	phasecontractPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	deliverablePkg   = "github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	policyPkg        = "github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	if err != nil {
		t.Fatalf("go test failed to launch for %s (%s): %v\nstderr:\n%s", pkg, pattern, err, stderr)
	}
	return code == 0, stdout + stderr
}

func TestC565_001_HandoffSummarySectionRequired(t *testing.T) {
	ok, out := runGoTest(t, phasecontractPkg,
		"TestHandoffSummarySection_Canonical|TestBuildScoutAudit_RequireHandoffSummary")
	if !ok {
		t.Errorf("phasecontract does not yet declare the canonical Handoff Summary section for build/scout/audit:\n%s", out)
	}
}

func TestC565_002_HandoffSummaryScopeBoundary(t *testing.T) {
	ok, out := runGoTest(t, phasecontractPkg, "TestTDDIntentTriage_NotExpanded")
	if !ok {
		t.Errorf("HandoffSummary must not leak into tdd/intent/triage this slice — scope creep beyond build/scout/audit:\n%s", out)
	}
}

func TestC565_003_TokenEstimateAndBudgetCheck(t *testing.T) {
	ok, out := runGoTest(t, deliverablePkg,
		"TestEstimateTokens|TestHandoffSectionContent|TestCheckHandoffBudget|TestCodeHandoffBudgetExceeded_IsStableIdentifier")
	if !ok {
		t.Errorf("deliverable package is missing the token-estimate/handoff-budget primitives:\n%s", out)
	}
}

func TestC565_004_ReportSizeGateShadowThenEnforce(t *testing.T) {
	ok, out := runGoTest(t, deliverablePkg,
		"TestReviewer_ReportSizeGate_BlocksOnlyAtEnforce|TestReviewer_ReportSizeGate_UnderBudgetNeverBlocks|"+
			"TestReviewer_ReportSizeGate_DefaultOff_ByteIdentical|TestVerifyWithReportSize_ShadowDoesNotViolate_EnforceDoes|"+
			"TestVerifyWithReportSize_UnderBudget_NeverViolates|TestVerifyWithReportSize_StageOff_EqualsVerifyWithStage")
	if !ok {
		t.Errorf("the report-size gate is not wired shadow-first-then-enforce at the Reviewer/Verify layer:\n%s", out)
	}
}

func TestC565_005_PolicyConfiguredBudgetDefaults(t *testing.T) {
	ok, out := runGoTest(t, policyPkg,
		"TestGatesConfig_ReportSizeGate_DefaultsShadow|TestGatesConfig_ReportSizeGate_ExplicitOverrideHonored|"+
			"TestReportBudgetConfig_DefaultsTo2000|TestReportBudgetConfig_ExplicitOverrideHonored|TestReportBudgetPolicy_JSONRoundTrip")
	if !ok {
		t.Errorf("policy package does not yet expose a config-driven ReportSizeGate/ReportBudget default:\n%s", out)
	}
}

//go:build acs

// Package cycle565 materialises the cycle-565 acceptance criteria for the
// fleet-lane-committed top_n task `report-size-contracts-jit-artifacts`
// (Slice S1 only — see triage-report.md and
// .evolve/inbox/2026-07-05T14-30-00Z-report-size-contracts-jit-artifacts.json).
//
// Slice S1 scope (per the triage decision): extend the existing contract-gate
// with a per-artifact handoff-summary token/size budget (default ~2K,
// policy-configured, shadow/warn first) and restructure the build/scout/audit
// phase report contracts with a never-evict "## Handoff Summary" section
// (decisions, acceptance criteria, open questions, verdicts). S2 (dependency
// pruning) and S3 (JIT read) are explicitly out of scope for this cycle.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549/553/555/557/
// 561/563 precedent) — each predicate shells `go test -run '^Name$' <pkg>`
// over the real RED unit tests authored this cycle in
// go/internal/phasecontract, go/internal/deliverable, and go/internal/policy.
// RED now (undefined identifiers — see test-report.md's RED Run Output);
// GREEN once Builder implements HandoffSummary/EstimateTokens/
// HandoffSectionContent/CheckHandoffBudget/VerifyWithReportSize/the Reviewer
// fields/GatesConfig.ReportSizeGate/Policy.ReportBudgetConfig.
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

// runGoTest shells `go test -run '^<pattern>$' -count=1 <pkg>` and returns
// whether it exited cleanly plus the combined output for diagnostics. -count=1
// defeats the test cache so the predicate always exercises current source.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	if err != nil {
		t.Fatalf("go test failed to launch for %s (%s): %v\nstderr:\n%s", pkg, pattern, err, stderr)
	}
	return code == 0, stdout + stderr
}

// TestC565_001_HandoffSummarySectionRequired — criterion 1: "Each phase
// report has a handoff-summary section validated against a policy-configured
// token budget" (the structural half). Drives TestHandoffSummarySection_Canonical
// and TestBuildScoutAudit_RequireHandoffSummary in go/internal/phasecontract.
func TestC565_001_HandoffSummarySectionRequired(t *testing.T) {
	ok, out := runGoTest(t, phasecontractPkg,
		"TestHandoffSummarySection_Canonical|TestBuildScoutAudit_RequireHandoffSummary")
	if !ok {
		t.Errorf("phasecontract does not yet declare the canonical Handoff Summary section for build/scout/audit:\n%s", out)
	}
}

// TestC565_002_HandoffSummaryScopeBoundary — negative/scope-guard companion
// to criterion 1: tdd/intent/triage must NOT gain the requirement this slice
// (S2/S3 territory or untouched). Drives TestTDDIntentTriage_NotExpanded.
func TestC565_002_HandoffSummaryScopeBoundary(t *testing.T) {
	ok, out := runGoTest(t, phasecontractPkg, "TestTDDIntentTriage_NotExpanded")
	if !ok {
		t.Errorf("HandoffSummary must not leak into tdd/intent/triage this slice — scope creep beyond build/scout/audit:\n%s", out)
	}
}

// TestC565_003_TokenEstimateAndBudgetCheck — criterion 1 (the size-estimation
// half): a deterministic token estimator plus a handoff-section budget check
// that flags oversized content and leaves an absent section alone. Drives
// TestEstimateTokens, TestHandoffSectionContent, TestCheckHandoffBudget, and
// TestCodeHandoffBudgetExceeded_IsStableIdentifier in go/internal/deliverable.
func TestC565_003_TokenEstimateAndBudgetCheck(t *testing.T) {
	ok, out := runGoTest(t, deliverablePkg,
		"TestEstimateTokens|TestHandoffSectionContent|TestCheckHandoffBudget|TestCodeHandoffBudgetExceeded_IsStableIdentifier")
	if !ok {
		t.Errorf("deliverable package is missing the token-estimate/handoff-budget primitives:\n%s", out)
	}
}

// TestC565_004_ReportSizeGateShadowThenEnforce — criterion 1 (the
// shadow/warn-first-then-enforce rollout half): the host-side contract gate
// (Reviewer) must not block on an oversized handoff section at shadow (or the
// zero-value/off default), but must block at enforce; VerifyWithReportSize
// must agree byte-identically with VerifyWithStage when the new dial is off.
// Drives the reviewer- and verify-layer RED tests in go/internal/deliverable.
func TestC565_004_ReportSizeGateShadowThenEnforce(t *testing.T) {
	ok, out := runGoTest(t, deliverablePkg,
		"TestReviewer_ReportSizeGate_BlocksOnlyAtEnforce|TestReviewer_ReportSizeGate_UnderBudgetNeverBlocks|"+
			"TestReviewer_ReportSizeGate_DefaultOff_ByteIdentical|TestVerifyWithReportSize_ShadowDoesNotViolate_EnforceDoes|"+
			"TestVerifyWithReportSize_UnderBudget_NeverViolates|TestVerifyWithReportSize_StageOff_EqualsVerifyWithStage")
	if !ok {
		t.Errorf("the report-size gate is not wired shadow-first-then-enforce at the Reviewer/Verify layer:\n%s", out)
	}
}

// TestC565_005_PolicyConfiguredBudgetDefaults — criterion 1 (the
// policy-configured half): the ~2K default budget and the gate's own
// shadow-default rollout stage must be resolved from .evolve/policy.json, not
// a Go literal reachable only through code (phase_settings_from_config_not_code).
// Drives the GatesConfig/ReportBudgetConfig RED tests in go/internal/policy.
func TestC565_005_PolicyConfiguredBudgetDefaults(t *testing.T) {
	ok, out := runGoTest(t, policyPkg,
		"TestGatesConfig_ReportSizeGate_DefaultsShadow|TestGatesConfig_ReportSizeGate_ExplicitOverrideHonored|"+
			"TestReportBudgetConfig_DefaultsTo2000|TestReportBudgetConfig_ExplicitOverrideHonored|TestReportBudgetPolicy_JSONRoundTrip")
	if !ok {
		t.Errorf("policy package does not yet expose a config-driven ReportSizeGate/ReportBudget default:\n%s", out)
	}
}

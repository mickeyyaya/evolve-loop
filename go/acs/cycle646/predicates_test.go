//go:build acs

// Package cycle646 materialises the acceptance criteria for cycle 646's two
// triage-committed top_n tasks with a genuinely new (not pre-existing) slice:
//
//   - report-handoff-size-contract-scout: cycle-565 Slice S1 already shipped
//     CheckHandoffBudget/VerifyWithReportSize/the Reviewer wiring, but left
//     "shadow" and "advisory" behaviourally identical (both fully silent) —
//     not the staged WARN-then-enforce rollout the source spec and this
//     cycle's scout-report Task 2 describe. This cycle's slice makes
//     "advisory" the missing WARN rung: it records the violation
//     (non-blocking) instead of staying silent.
//   - persona-stop-criterion-dedupe: agents/evolve-{scout,builder,auditor}.md
//     each duplicate a structurally-identical "## STOP CRITERION" block with
//     zero shared wording (751 combined lines) — extract the shared structure
//     into one reference doc without losing any gate name or banned pattern.
//
// Task 1 (cache-stable-prompt-prefix-audit) is NOT predicated here: its
// underlying mechanism (go/internal/phases/runner's cycleContextBoundary /
// BaseCycleContext / StaticPrefix, cycle-535) is already shipped and covered
// by go/internal/phases/runner/staticprefix_test.go (pre-existing GREEN). The
// one remaining ordering gap — go/internal/adapters/bridge.Adapter.Launch
// puts CorrectionDirective/OperatorDirectives OUTERMOST, ahead of the static
// Rules/Policy/Contract/persona block — is INTENTIONAL, tested behavior
// (bridge_correction_test.go: TestCorrectionDirectiveComposesWithRules /
// TestLaunch_InjectsCorrectionBlock; bridge_directives_test.go:
// TestOperatorDirectivesComposeOrder / TestLaunch_InjectsOperatorDirectives —
// all assert "correction < directives < rules < body" as the REQUIRED order,
// for retry/directive salience). Inverting it to satisfy a literal
// "static-always-precedes-dynamic" reading would regress that shipped,
// tested salience feature. See test-report.md's disposition table — flagged
// manual+checklist for Auditor, not predicated.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…637
// precedent) — each predicate shells `go test -run '^Name$' <pkg>` over the
// real RED unit tests authored this cycle. RED now: TestVerifyWithReportSize_
// AdvisoryRecordsWarnViolation (deliverable) and TestPersonaStopCriterionDedupe_
// CombinedLineCountReduced (prompts) fail; see test-report.md's RED Run
// Output for the other three (pre-existing GREEN — they already hold and
// serve as regression/scope guards for the Builder's change).
package cycle646

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	deliverablePkg = "github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	promptsPkg     = "github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the
// test cache so the predicate always exercises current source. code<0 is a
// genuine launch failure (binary missing / killed by signal), never a test
// verdict — that fails loudly rather than being misread as RED.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC646_001_ReportSizeGateAdvisoryWarns — Task 2's genuinely new slice: the
// missing WARN rung. RED today (VerifyWithReportSize's early-return threshold
// is `< StageEnforce`, so advisory stays silent).
func TestC646_001_ReportSizeGateAdvisoryWarns(t *testing.T) {
	ok, out := runGoTest(t, deliverablePkg, "TestVerifyWithReportSize_AdvisoryRecordsWarnViolation")
	if !ok {
		t.Errorf("advisory (WARN) rung not yet recording the handoff-budget violation:\n%s", out)
	}
}

// TestC646_002_ReportSizeGateShadowStaysSilent — negative/scope guard: shadow
// must remain fully dormant (cycle-565 contract) — only advisory gains WARN
// behavior this cycle. Pre-existing GREEN; guards against an over-broad fix.
func TestC646_002_ReportSizeGateShadowStaysSilent(t *testing.T) {
	ok, out := runGoTest(t, deliverablePkg, "TestVerifyWithReportSize_ShadowStaysSilent_Negative")
	if !ok {
		t.Errorf("shadow rung must stay silent (dormant) — WARN is advisory-only:\n%s", out)
	}
}

// TestC646_003_ReviewerAdvisoryWarnsButApproves — host-side effect: an
// oversized Handoff Summary at reportSizeGate=advisory must approve (never
// block), mirroring enforce's blocking counterpart. Pre-existing GREEN.
func TestC646_003_ReviewerAdvisoryWarnsButApproves(t *testing.T) {
	ok, out := runGoTest(t, deliverablePkg, "TestReviewer_ReportSizeGate_AdvisoryWarnsButApproves")
	if !ok {
		t.Errorf("reviewer must approve (non-blocking) at reportSizeGate=advisory:\n%s", out)
	}
}

// TestC646_004_PersonaCombinedLineCountReduced — Task 3's primary signal: a
// no-op dedupe (nothing extracted) must fail. RED today (751 combined lines,
// the pre-dedupe baseline).
func TestC646_004_PersonaCombinedLineCountReduced(t *testing.T) {
	ok, out := runGoTest(t, promptsPkg, "TestPersonaStopCriterionDedupe_CombinedLineCountReduced")
	if !ok {
		t.Errorf("combined evolve-scout/builder/auditor.md line count not yet reduced below the 751-line pre-dedupe baseline:\n%s", out)
	}
}

// TestC646_005_PersonaNoGateOrBannedPatternTextLost — negative/scope guard:
// every named completion gate and banned-post-report phrase must survive the
// dedupe somewhere under agents/. Pre-existing GREEN; guards against a
// lossy extraction.
func TestC646_005_PersonaNoGateOrBannedPatternTextLost(t *testing.T) {
	ok, out := runGoTest(t, promptsPkg, "TestPersonaStopCriterionDedupe_NoGateOrBannedPatternTextLost")
	if !ok {
		t.Errorf("a gate name or banned-post-report phrase was lost from agents/evolve-*.md during dedupe:\n%s", out)
	}
}

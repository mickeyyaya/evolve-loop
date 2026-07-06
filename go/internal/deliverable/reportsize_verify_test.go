package deliverable

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// reportsize_verify_test.go — RED contract for the VerifyWithStage→next-layer
// threading of the new report-size gate, mirroring exactly how PhaseIO was
// added as a new layer over VerifyWith (deliverable.go's own precedent) rather
// than changing VerifyWithStage's signature (which would ripple through every
// existing caller: cmd_phase_verify.go, reviewer.go, verifier.go,
// catalogaware.go). VerifyWithReportSize is VerifyWithStage plus one more
// stage-gated check, exactly as VerifyWithStage is VerifyWith plus phaseIO.
//
// RED today: VerifyWithReportSize does not exist (compile failure).

func TestVerifyWithReportSize_ShadowDoesNotViolate_EnforceDoes(t *testing.T) {
	ws := t.TempDir()
	big := strings.Repeat("word ", 5000)
	report := "## Changes\n- x\nVerdict: PASS\n## Handoff Summary\n" + big
	writeFile(t, ws, "build-report.md", report)
	roots := phasecontract.Roots{Workspace: ws}

	shadow, err := VerifyWithReportSize("build", roots, phasecontract.BuiltinResolver{}, config.StageOff, config.StageShadow, 2000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasCode(shadow, CodeHandoffBudgetExceeded) {
		t.Errorf("reportSizeGate=shadow must not add %s to Violations (dormant, log-only); got %+v", CodeHandoffBudgetExceeded, shadow.Violations)
	}

	enforce, err := VerifyWithReportSize("build", roots, phasecontract.BuiltinResolver{}, config.StageOff, config.StageEnforce, 2000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasCode(enforce, CodeHandoffBudgetExceeded) {
		t.Errorf("reportSizeGate=enforce must add %s when the handoff section exceeds budget; got %+v", CodeHandoffBudgetExceeded, enforce.Violations)
	}
}

func TestVerifyWithReportSize_UnderBudget_NeverViolates(t *testing.T) {
	ws := t.TempDir()
	report := "## Changes\n- x\nVerdict: PASS\n## Handoff Summary\nshort decision\n"
	writeFile(t, ws, "build-report.md", report)
	roots := phasecontract.Roots{Workspace: ws}

	res, err := VerifyWithReportSize("build", roots, phasecontract.BuiltinResolver{}, config.StageOff, config.StageEnforce, 2000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasCode(res, CodeHandoffBudgetExceeded) {
		t.Errorf("a handoff section within budget must never violate, even at enforce; got %+v", res.Violations)
	}
}

// TestVerifyWithReportSize_StageOff_EqualsVerifyWithStage pins byte-identical
// behavior when the new dial is off (default): VerifyWithReportSize must agree
// exactly with the pre-existing VerifyWithStage on both OK and Violations, so
// wiring the new layer in cannot silently change existing gate behavior for
// every cycle that has not opted into the new gate.
func TestVerifyWithReportSize_StageOff_EqualsVerifyWithStage(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "build-report.md", "## Changes\n- x\nVerdict: PASS\n")
	roots := phasecontract.Roots{Workspace: ws}

	legacy, err := VerifyWithStage("build", roots, phasecontract.BuiltinResolver{}, config.StageOff)
	if err != nil {
		t.Fatalf("VerifyWithStage: %v", err)
	}
	next, err := VerifyWithReportSize("build", roots, phasecontract.BuiltinResolver{}, config.StageOff, config.StageOff, 2000)
	if err != nil {
		t.Fatalf("VerifyWithReportSize: %v", err)
	}
	if legacy.OK != next.OK || len(legacy.Violations) != len(next.Violations) {
		t.Errorf("VerifyWithReportSize(reportSizeGate=off) diverged from VerifyWithStage: legacy=%+v next=%+v", legacy, next)
	}
}

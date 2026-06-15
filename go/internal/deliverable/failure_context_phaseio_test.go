package deliverable

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// Phase 3.8 (ADR-0050): the structured-failure-block requirement — enforced
// today only for audit via the unconditional Contract.RequireFailureContext —
// is generalized to build/scout/triage, but gated on the EVOLVE_PHASE_IO dial
// so it is byte-identical (dormant) until enforce. A non-audit phase that
// self-reports a FAIL/WARN verdict sentinel WITHOUT a structured failure block
// is a violation ONLY at PhaseIO>=enforce; at off/shadow/advisory it does not
// fire. PASS sentinels and legacy prose-only artifacts stay legal forever.

// phaseioFailFixtures maps each generalized phase to its artifact name and one
// required section, so a fixture report is section-complete and the ONLY
// possible violation is the failure-context one.
var phaseioFailFixtures = map[string]struct {
	artifact string
	section  string
}{
	"build":  {"build-report.md", "## Changes"},
	"scout":  {"scout-report.md", "## Selected Tasks"},
	"triage": {"triage-report.md", "## top_n"},
}

// failReport returns a section-complete <phase> report whose verdict sentinel
// declares FAIL. withBlock controls whether the structured failure block is
// present.
func failReport(phase, section string, withBlock bool) string {
	var line string
	if withBlock {
		line = phasecontract.RenderVerdictSentinelWithFailure(phase, "FAIL",
			&phasecontract.FailureBlock{Class: "code-" + phase + "-fail", Defects: []string{"d1"}})
	} else {
		line = phasecontract.RenderVerdictSentinel(phase, "FAIL")
	}
	return "# Report\n\n" + section + "\n- x\n\n" + line + "\n"
}

func TestVerifyWithStage_NonAuditFailWithoutBlock_BlocksAtEnforce(t *testing.T) {
	for phase, fx := range phaseioFailFixtures {
		t.Run(phase, func(t *testing.T) {
			ws := t.TempDir()
			writeFile(t, ws, fx.artifact, failReport(phase, fx.section, false))
			res, err := VerifyWithStage(phase, phasecontract.Roots{Workspace: ws}, phasecontract.BuiltinResolver{}, config.StageEnforce)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.OK || !hasCode(res, CodeFailureContextMissing) {
				t.Fatalf("phase %s at enforce: want failure_context_missing, got %+v", phase, res.Violations)
			}
		})
	}
}

func TestVerifyWithStage_NonAuditFailWithoutBlock_DormantBelowEnforce(t *testing.T) {
	for _, stage := range []config.Stage{config.StageOff, config.StageShadow, config.StageAdvisory} {
		for phase, fx := range phaseioFailFixtures {
			t.Run(stage.String()+"/"+phase, func(t *testing.T) {
				ws := t.TempDir()
				writeFile(t, ws, fx.artifact, failReport(phase, fx.section, false))
				res, err := VerifyWithStage(phase, phasecontract.Roots{Workspace: ws}, phasecontract.BuiltinResolver{}, stage)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !res.OK {
					t.Fatalf("phase %s at stage %s: failure-context check must be dormant, got %+v", phase, stage, res.Violations)
				}
			})
		}
	}
}

// WARN is a first-class gated verdict too — the clause checks (FAIL || WARN), so
// a WARN-without-block sentinel must also block at enforce. Without this, the
// `|| s.Verdict == "WARN"` disjunction is an untested surviving mutant.
func TestVerifyWithStage_NonAuditWarnWithoutBlock_BlocksAtEnforce(t *testing.T) {
	for phase, fx := range phaseioFailFixtures {
		t.Run(phase, func(t *testing.T) {
			ws := t.TempDir()
			content := "# Report\n\n" + fx.section + "\n- x\n\n" + phasecontract.RenderVerdictSentinel(phase, "WARN") + "\n"
			writeFile(t, ws, fx.artifact, content)
			res, err := VerifyWithStage(phase, phasecontract.Roots{Workspace: ws}, phasecontract.BuiltinResolver{}, config.StageEnforce)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.OK || !hasCode(res, CodeFailureContextMissing) {
				t.Fatalf("phase %s WARN-without-block at enforce: want failure_context_missing, got %+v", phase, res.Violations)
			}
		})
	}
}

func TestVerifyWithStage_NonAuditFailWithBlock_OKAtEnforce(t *testing.T) {
	for phase, fx := range phaseioFailFixtures {
		t.Run(phase, func(t *testing.T) {
			ws := t.TempDir()
			writeFile(t, ws, fx.artifact, failReport(phase, fx.section, true))
			res, err := VerifyWithStage(phase, phasecontract.Roots{Workspace: ws}, phasecontract.BuiltinResolver{}, config.StageEnforce)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !res.OK {
				t.Fatalf("phase %s with structured FAIL block at enforce: want OK, got %+v", phase, res.Violations)
			}
		})
	}
}

// PASS sentinels never require a failure block, even at enforce — only FAIL/WARN
// bites (the registry comment's "never false-block" invariant for these phases).
func TestVerifyWithStage_NonAuditPass_OKAtEnforce(t *testing.T) {
	for phase, fx := range phaseioFailFixtures {
		t.Run(phase, func(t *testing.T) {
			ws := t.TempDir()
			content := "# Report\n\n" + fx.section + "\n- x\n\n" + phasecontract.RenderVerdictSentinel(phase, "PASS") + "\n"
			writeFile(t, ws, fx.artifact, content)
			res, err := VerifyWithStage(phase, phasecontract.Roots{Workspace: ws}, phasecontract.BuiltinResolver{}, config.StageEnforce)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !res.OK {
				t.Fatalf("phase %s PASS at enforce: want OK, got %+v", phase, res.Violations)
			}
		})
	}
}

// VerifyWith is exactly VerifyWithStage at StageOff — the byte-identical default
// that keeps every existing caller (and the `evolve phase verify` self-check)
// unchanged.
func TestVerifyWith_EqualsStageOff(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "build-report.md", failReport("build", "## Changes", false))
	roots := phasecontract.Roots{Workspace: ws}
	a, errA := VerifyWith("build", roots, phasecontract.BuiltinResolver{})
	b, errB := VerifyWithStage("build", roots, phasecontract.BuiltinResolver{}, config.StageOff)
	if errA != nil || errB != nil {
		t.Fatalf("errs: %v %v", errA, errB)
	}
	if a.OK != b.OK || len(a.Violations) != len(b.Violations) {
		t.Fatalf("VerifyWith must equal VerifyWithStage(StageOff): %+v vs %+v", a, b)
	}
	if !a.OK {
		t.Fatalf("at StageOff a build FAIL-without-block must be OK (dormant), got %+v", a.Violations)
	}
}

// Audit's unconditional RequireFailureContext is independent of the PhaseIO dial
// — it still fires at StageOff (byte-identical to pre-3.8).
func TestVerifyWithStage_AuditFailWithoutBlock_FiresRegardlessOfStage(t *testing.T) {
	for _, stage := range []config.Stage{config.StageOff, config.StageEnforce} {
		ws := t.TempDir()
		writeFile(t, ws, "audit-report.md", "## Verdict\nFAIL\n"+phasecontract.RenderVerdictSentinel("audit", "FAIL")+"\n")
		res, err := VerifyWithStage("audit", phasecontract.Roots{Workspace: ws}, phasecontract.BuiltinResolver{}, stage)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.OK || !hasCode(res, CodeFailureContextMissing) {
			t.Fatalf("audit at stage %s: RequireFailureContext is unconditional, want failure_context_missing, got %+v", stage, res.Violations)
		}
	}
}

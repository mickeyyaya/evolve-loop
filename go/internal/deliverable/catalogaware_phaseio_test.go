package deliverable

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// Empty roots.EvolveDir takes the BuiltinResolver branch, so these match VerifyWithStage for built-in phases.
func TestVerifyCatalogAwareStage_FailWithoutBlock_BlocksAtEnforce(t *testing.T) {
	for phase, fx := range phaseioFailFixtures {
		t.Run(phase, func(t *testing.T) {
			ws := t.TempDir()
			writeFile(t, ws, fx.artifact, failReport(phase, fx.section, false))
			res, err := VerifyCatalogAwareStage(phase, phasecontract.Roots{Workspace: ws}, config.StageEnforce)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.OK || !hasCode(res, CodeFailureContextMissing) {
				t.Fatalf("phase %s at enforce: want failure_context_missing, got %+v", phase, res.Violations)
			}
		})
	}
}

func TestVerifyCatalogAwareStage_FailWithoutBlock_DormantBelowEnforce(t *testing.T) {
	for _, stage := range []config.Stage{config.StageOff, config.StageShadow, config.StageAdvisory} {
		for phase, fx := range phaseioFailFixtures {
			t.Run(stage.String()+"/"+phase, func(t *testing.T) {
				ws := t.TempDir()
				writeFile(t, ws, fx.artifact, failReport(phase, fx.section, false))
				res, err := VerifyCatalogAwareStage(phase, phasecontract.Roots{Workspace: ws}, stage)
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

func TestVerifyCatalogAware_EqualsStageOff(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "build-report.md", failReport("build", "## Changes", false))
	roots := phasecontract.Roots{Workspace: ws}
	a, errA := VerifyCatalogAware("build", roots)
	b, errB := VerifyCatalogAwareStage("build", roots, config.StageOff)
	if errA != nil || errB != nil {
		t.Fatalf("errs: %v %v", errA, errB)
	}
	if a.OK != b.OK || !equalViolationCodes(a, b) {
		t.Fatalf("VerifyCatalogAware must equal VerifyCatalogAwareStage(StageOff): %+v vs %+v", a, b)
	}
	if !a.OK {
		t.Fatalf("at StageOff a build FAIL-without-block must be OK (dormant), got %+v", a.Violations)
	}
}

// equalViolationCodes compares ordered violation codes, which is stricter than comparing counts.
func equalViolationCodes(a, b Result) bool {
	if len(a.Violations) != len(b.Violations) {
		return false
	}
	for i := range a.Violations {
		if a.Violations[i].Code != b.Violations[i].Code {
			return false
		}
	}
	return true
}

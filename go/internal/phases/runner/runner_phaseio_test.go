package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// TestNew_DefaultVerifyFnHonorsPhaseIO (ADR-0050 Phase 3.10 Slice 1): the
// reconcile-on-timeout default verifyFn now threads Options.PhaseIO into the
// catalog-aware check, so at enforce a build FAIL-without-block artifact is caught
// (parity with the host gate) and below enforce it stays dormant (byte-identical).
func TestNew_DefaultVerifyFnHonorsPhaseIO(t *testing.T) {
	ws := t.TempDir()
	// A section-complete build report whose only possible violation is the
	// missing failure block (FAIL sentinel without a structured block).
	report := "# Report\n\n## Changes\n- x\n\n" + phasecontract.RenderVerdictSentinel("build", "FAIL") + "\n"
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	roots := phasecontract.Roots{Workspace: ws} // EvolveDir "" => BuiltinResolver path

	enforce := New(Options{Hooks: &fakeHooks{phase: "build", verdict: core.VerdictPASS}, PhaseIO: config.StageEnforce})
	resE, err := enforce.verifyFn("build", roots)
	if err != nil {
		t.Fatalf("enforce verifyFn err: %v", err)
	}
	if resE.OK {
		t.Errorf("at PhaseIO=enforce the default reconcile verifyFn must catch a build FAIL-without-block, got OK")
	}

	off := New(Options{Hooks: &fakeHooks{phase: "build", verdict: core.VerdictPASS}})
	resO, err := off.verifyFn("build", roots)
	if err != nil {
		t.Fatalf("off verifyFn err: %v", err)
	}
	if !resO.OK {
		t.Errorf("at PhaseIO=off (default) the reconcile verifyFn must stay dormant (byte-identical), got blocked: %+v", resO.Violations)
	}
}

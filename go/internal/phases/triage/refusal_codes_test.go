package triage

// refusal_codes_test.go — the three deterministic refusals Classify itself
// raises (not the agent's verdict) carry a stable Diagnostic.Code, so the C1
// record and the FAIL closeout can tell "the item is operator-owned" from
// "the agent had a bad day" without regexing prose
// (docs/incidents/2026-09-14-triage-refusal-poison-loop.md).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
)

func errorCodesOf(diags []core.Diagnostic) []string {
	var out []string
	for _, d := range diags {
		if d.Severity == cyclestate.SeverityError {
			out = append(out, d.Code)
		}
	}
	return out
}

func TestTriageClassify_ProtectedSurfaceRefusalCarriesItsCode(t *testing.T) {
	if !guards.IsProtectedSurface("go/acs/regression/cycle1/predicates_test.go") {
		t.Fatal("pin moved: go/acs/regression/ no longer on ProtectedSurfaceManifest")
	}
	artifact := "## top_n\n- tamper: rewrite a predicate — priority=H, files={go/acs/regression/cycle1/predicates_test.go}, source=scout\n"
	verdict, diags, _ := hooks{}.Classify(artifact, core.PhaseRequest{}, core.BridgeResponse{})
	if verdict != core.VerdictFAIL {
		t.Fatalf("verdict = %s", verdict)
	}
	if codes := errorCodesOf(diags); len(codes) != 1 || codes[0] != cyclestate.DiagCodeTriageProtectedSurface {
		t.Errorf("codes = %v, want [%s]; diags = %+v", codes, cyclestate.DiagCodeTriageProtectedSurface, diags)
	}
}

func TestTriageClassify_EmptyTopNRefusalCarriesItsCode(t *testing.T) {
	verdict, diags, _ := hooks{}.Classify("## top_n\n\nnothing listed here\n", core.PhaseRequest{}, core.BridgeResponse{})
	if verdict != core.VerdictFAIL {
		t.Fatalf("verdict = %s", verdict)
	}
	if codes := errorCodesOf(diags); len(codes) != 1 || codes[0] != cyclestate.DiagCodeTriageTopNEmpty {
		t.Errorf("codes = %v, want [%s]", codes, cyclestate.DiagCodeTriageTopNEmpty)
	}
}

func TestTriageClassify_CommitmentInvalidRefusalCarriesItsCode(t *testing.T) {
	ws := t.TempDir()
	// A directory where triage-decision.json belongs: the read faults (not absence).
	if err := os.MkdirAll(filepath.Join(ws, "triage-decision.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := "## top_n\n- ok-card: a plain card — priority=M, files={go/internal/foo/foo.go}, source=scout\n"
	verdict, diags, _ := hooks{}.Classify(artifact, core.PhaseRequest{Workspace: ws}, core.BridgeResponse{})
	if verdict != core.VerdictFAIL {
		t.Fatalf("verdict = %s (a faulting decision read must refuse)", verdict)
	}
	if codes := errorCodesOf(diags); len(codes) != 1 || codes[0] != cyclestate.DiagCodeTriageCommitmentInvalid {
		t.Errorf("codes = %v, want [%s]; diags = %+v", codes, cyclestate.DiagCodeTriageCommitmentInvalid, diags)
	}
}

// A PASS carries no refusal code — the vocabulary is for Classify's own gates.
func TestTriageClassify_PassCarriesNoRefusalCode(t *testing.T) {
	artifact := "## top_n\n- ok-card: a plain card — priority=M, files={go/internal/foo/foo.go}, source=scout\n"
	verdict, diags, _ := hooks{}.Classify(artifact, core.PhaseRequest{Workspace: t.TempDir()}, core.BridgeResponse{})
	if verdict != core.VerdictPASS {
		t.Fatalf("verdict = %s; diags = %+v", verdict, diags)
	}
	for _, d := range diags {
		if d.Code != "" {
			t.Errorf("a PASS diagnostic must not carry a refusal code: %+v", d)
		}
	}
}

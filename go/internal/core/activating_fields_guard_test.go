package core

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestRegistryActivatingFields_AgreeWithLiteralKernel(t *testing.T) {
	t.Parallel()
	cat, err := phasespec.Load(filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Fatalf("load shipped registry: %v", err)
	}

	retro, ok := cat.Get(canonicalCatalogName(PhaseRetro))
	if !ok {
		t.Fatalf("shipped registry missing %q", canonicalCatalogName(PhaseRetro))
	}
	if want := literalSuccessorStrategy(PhaseRetro); retro.BranchingStrategy != want {
		t.Errorf("retrospective branching_strategy = %q, want %q (literal backstop) — activating field dropped or drifted",
			retro.BranchingStrategy, want)
	}

	audit, ok := cat.Get(canonicalCatalogName(PhaseAudit))
	if !ok {
		t.Fatalf("shipped registry missing %q", canonicalCatalogName(PhaseAudit))
	}
	if audit.OnPass == "" || audit.OnFail == "" {
		t.Fatalf("audit must declare on_pass+on_fail (verdict branch); got on_pass=%q on_fail=%q — activating field dropped",
			audit.OnPass, audit.OnFail)
	}
	bare := NewStateMachine() // catalog-less = literal authority
	wantPass, _ := bare.Next(PhaseAudit, VerdictPASS)
	wantFail, _ := bare.Next(PhaseAudit, VerdictFAIL)
	if got := phaseFromRouter(audit.OnPass); got != wantPass {
		t.Errorf("audit on_pass=%q resolves to %q, want literal %q", audit.OnPass, got, wantPass)
	}
	if got := phaseFromRouter(audit.OnFail); got != wantFail {
		t.Errorf("audit on_fail=%q resolves to %q, want literal %q", audit.OnFail, got, wantFail)
	}
}

func TestControlSeamActivatingFields_AgreeWithLiteralKernel(t *testing.T) {
	t.Parallel()
	spec, ok := builtinControlSpec(PhaseDebugger)
	if !ok {
		t.Fatal("control seam must describe the debugger")
	}
	if want := literalSuccessorStrategy(PhaseDebugger); spec.BranchingStrategy != want {
		t.Errorf("debugger seam branching_strategy = %q, want %q (literal backstop) — seam and literal drifted",
			spec.BranchingStrategy, want)
	}
}

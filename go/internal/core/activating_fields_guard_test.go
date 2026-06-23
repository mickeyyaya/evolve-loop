package core

// activating_fields_guard_test.go — PA-BIG S4 (ADR-0058): the hard registry
// guard (trust anchor). ADR-0058 made the kernel config-driven with a
// byte-identical LITERAL backstop, so dropping an activating field from the
// shipped registry would silently revert that phase to literal-as-SSOT with NO
// observable behavior change — invisible to every behavior test. This guard
// makes the drift LOUD by asserting the shipped registry (and the control seam)
// AGREE with the literal kernel: config must stay the live source, the literal a
// pure backstop. Expectations are DERIVED from the literal (not a fresh golden),
// so the guard catches drops AND divergence.

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/phasespec"
)

// TestRegistryActivatingFields_AgreeWithLiteralKernel loads the shipped registry
// and asserts the ADR-0058 activating fields are present and resolve to the same
// successors the literal kernel would pick.
func TestRegistryActivatingFields_AgreeWithLiteralKernel(t *testing.T) {
	t.Parallel()
	cat, err := phasespec.Load(filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Fatalf("load shipped registry: %v", err)
	}

	// retrospective: the registry's branching_strategy must equal the literal
	// backstop (history). A dropped/changed field fails here.
	retro, ok := cat.Get(canonicalCatalogName(PhaseRetro))
	if !ok {
		t.Fatalf("shipped registry missing %q", canonicalCatalogName(PhaseRetro))
	}
	if want := literalSuccessorStrategy(PhaseRetro); retro.BranchingStrategy != want {
		t.Errorf("retrospective branching_strategy = %q, want %q (literal backstop) — activating field dropped or drifted",
			retro.BranchingStrategy, want)
	}

	// audit: the registry must declare BOTH verdict-branch targets, and they must
	// resolve to the same phases the literal Next() picks for PASS and FAIL.
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

// TestControlSeamActivatingFields_AgreeWithLiteralKernel is the seam half of the
// guard: the debugger has no registry home, so its branching_strategy lives in
// the builtinControlSpec seam. That seam value must equal the literal backstop —
// the two Go sources of the debugger strategy cannot drift apart.
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

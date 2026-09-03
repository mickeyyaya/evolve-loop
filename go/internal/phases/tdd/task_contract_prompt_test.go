package tdd

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestComposePrompt_RendersTaskContractVerbatim — tdd gets the same
// harness-owned acceptance block the build gets (predicates come later).
func TestComposePrompt_RendersTaskContractVerbatim(t *testing.T) {
	contract := "### task-a — Title A\nAcceptance (verbatim from the inbox item — the auditor grades against exactly these):\n1. a failing test encodes each criterion\n\n"
	got := hooks{}.ComposePrompt("body", core.PhaseRequest{Context: map[string]string{core.CtxKeyTaskContract: contract}})
	if !strings.Contains(got, "## Task Contract") || !strings.Contains(got, contract) {
		t.Fatalf("Task Contract block must be rendered verbatim under its heading:\n%s", got)
	}
	if got := (hooks{}).ComposePrompt("body", core.PhaseRequest{}); strings.Contains(got, "## Task Contract") {
		t.Fatal("no contract seeded ⇒ no heading")
	}
}

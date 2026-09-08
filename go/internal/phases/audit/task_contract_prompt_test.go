package audit

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestComposePrompt_RendersTaskContractVerbatim — the grader reads the same
// harness-owned block the builder was handed (ADR-0098); absent key ⇒ no heading.
func TestComposePrompt_RendersTaskContractVerbatim(t *testing.T) {
	contract := "### task-a — Title A\nAcceptance (verbatim from the inbox item — the auditor grades against exactly these):\n1. graded verbatim\n\n### ACS predicates (harness-listed via `go test -list . -tags acs`; every one must be GREEN before the build hands off)\n- TestC7_001_X\n\n"
	got := (hooks{}).ComposePrompt("PERSONA BODY", core.PhaseRequest{Context: map[string]string{core.CtxKeyTaskContract: contract}})
	if !strings.Contains(got, "## Task Contract") || !strings.Contains(got, contract) {
		t.Fatalf("audit must render the Task Contract verbatim under its heading:\n%s", got)
	}
	if got := (hooks{}).ComposePrompt("PERSONA BODY", core.PhaseRequest{}); strings.Contains(got, "## Task Contract") {
		t.Fatal("no contract seeded ⇒ no heading")
	}
}

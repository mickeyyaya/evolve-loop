package build

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestComposePrompt_RendersTaskContractVerbatim — the harness-owned block
// reaches the builder under its own heading, verbatim, and only when seeded.
func TestComposePrompt_RendersTaskContractVerbatim(t *testing.T) {
	contract := "### task-a — Title A\nAcceptance (verbatim from the inbox item — the auditor grades against exactly these):\n1. build-prompt.txt carries acceptance[] verbatim\n\n### ACS predicates (harness-listed via `go test -list . -tags acs`; every one must be GREEN before handoff)\n- TestC7_001_X\n\n"
	got := hooks{}.ComposePrompt("body", core.PhaseRequest{Context: map[string]string{core.CtxKeyTaskContract: contract}})
	if !strings.Contains(got, "## Task Contract") || !strings.Contains(got, contract) {
		t.Fatalf("Task Contract block must be rendered verbatim under its heading:\n%s", got)
	}
	if strings.Index(got, "## Task Contract") < strings.Index(got, "- cycle:") {
		t.Fatal("the block belongs after the cycle context, with the other harness-owned blocks")
	}
	if got := (hooks{}).ComposePrompt("body", core.PhaseRequest{}); strings.Contains(got, "## Task Contract") {
		t.Fatal("no contract seeded ⇒ no heading (byte-identical prompt)")
	}
}

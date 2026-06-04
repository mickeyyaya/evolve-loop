package bridge

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// bridge_correction_test.go — contract-correction retry: the "## Correction"
// prompt block carrying the orchestrator's re-dispatch directive. Applied at
// the same CLI-agnostic seam as injectRulesPrefix, OUTERMOST.

func TestInjectCorrectionPrefix(t *testing.T) {
	// Empty directive = identity (off path byte-identical).
	if got := injectCorrectionPrefix("BODY", ""); got != "BODY" {
		t.Errorf("empty directive must pass through, got %q", got)
	}
	// Non-empty prepends a ## Correction block above the body.
	got := injectCorrectionPrefix("BODY", "fix the Verdict section")
	if !strings.HasPrefix(got, "## Correction\n\n") {
		t.Errorf("missing Correction header: %q", got)
	}
	if !strings.Contains(got, "fix the Verdict section") || !strings.HasSuffix(got, "BODY") {
		t.Errorf("directive/body not assembled: %q", got)
	}
}

// End-to-end: a BridgeRequest's CorrectionDirective rides through the same
// prompt-assembly seam as SystemPrompt, landing OUTERMOST (above ## Rules).
func TestCorrectionDirectiveComposesWithRules(t *testing.T) {
	var req core.BridgeRequest
	req.SystemPrompt = "RULE TEXT"
	req.CorrectionDirective = "rewrite the Verdict section"

	// Mirror the adapter's :125 assembly order.
	withRules := injectRulesPrefix("BODY", req.SystemPrompt)
	composed := injectCorrectionPrefix(withRules, req.CorrectionDirective)

	corrIdx := strings.Index(composed, "## Correction")
	rulesIdx := strings.Index(composed, "## Rules")
	bodyIdx := strings.Index(composed, "BODY")
	if corrIdx != 0 {
		t.Fatalf("correction block must lead; composed=%q", composed)
	}
	if !(corrIdx < rulesIdx && rulesIdx < bodyIdx) {
		t.Fatalf("order must be correction<rules<body; got corr=%d rules=%d body=%d", corrIdx, rulesIdx, bodyIdx)
	}
	if !strings.Contains(composed, "rewrite the Verdict section") {
		t.Fatalf("directive text missing: %q", composed)
	}

	// Off path: empty CorrectionDirective leaves the assembly unchanged.
	var off core.BridgeRequest
	off.SystemPrompt = "RULE TEXT"
	if got := injectCorrectionPrefix(injectRulesPrefix("BODY", off.SystemPrompt), off.CorrectionDirective); got != withRules {
		t.Fatalf("empty directive must be a no-op; got %q want %q", got, withRules)
	}
}

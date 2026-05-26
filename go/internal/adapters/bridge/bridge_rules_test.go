package bridge

import (
	"strings"
	"testing"
)

// bridge_rules_test.go — facet B: launch-time system-prompt/rules prepend.
// CLI-agnostic, composed with the interactive-policy block at the same seam.

func TestInjectRulesPrefix_EmptyRulesUnchanged(t *testing.T) {
	if got := injectRulesPrefix("body", ""); got != "body" {
		t.Errorf("empty rules should pass through unchanged; got %q", got)
	}
}

func TestInjectRulesPrefix_PrependsRulesBlock(t *testing.T) {
	got := injectRulesPrefix("PROMPT BODY", "be terse and cite sources")
	if !strings.HasPrefix(got, "## Rules") {
		t.Errorf("rules block should lead; got first 40 chars: %q", got[:min(40, len(got))])
	}
	if !strings.Contains(got, "be terse and cite sources") {
		t.Errorf("rules text missing; got %q", got)
	}
	if !strings.HasSuffix(got, "PROMPT BODY") {
		t.Errorf("original body should be preserved at the end; got %q", got)
	}
}

// Composition order: rules block precedes the policy block, which precedes
// the body — the exact order the adapter applies at both launch seams.
func TestRulesAndPolicy_ComposeInOrder(t *testing.T) {
	withPolicy := injectPolicyPrefix("BODY", PolicyRecommendedOrFirst)
	composed := injectRulesPrefix(withPolicy, "RULE TEXT")

	rulesIdx := strings.Index(composed, "## Rules")
	policyIdx := strings.Index(composed, "## Subagent Interactive Policy")
	bodyIdx := strings.Index(composed, "BODY")
	if rulesIdx != 0 {
		t.Fatalf("rules block must lead; composed=%q", truncate(composed, 80))
	}
	if !(rulesIdx < policyIdx && policyIdx < bodyIdx) {
		t.Fatalf("order must be rules<policy<body; got rules=%d policy=%d body=%d", rulesIdx, policyIdx, bodyIdx)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

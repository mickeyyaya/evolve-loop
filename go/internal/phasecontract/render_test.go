package phasecontract

import (
	"strings"
	"testing"
)

// Layer 2 (ADR-0034): the Deliverable Contract is rendered into the prompt in
// two pieces — an INVARIANT instruction block (stable cache prefix, no path) and
// a VOLATILE path footer (last line; recency-optimal AND it keeps the per-cycle
// path out of the cacheable prefix).

func TestRenderContractBlock_Markdown_NoPath(t *testing.T) {
	c, _ := For("build")
	block := RenderContractBlock(c)
	if !strings.Contains(block, "## Changes") {
		t.Errorf("block must name required section %q; got:\n%s", "## Changes", block)
	}
	if !strings.Contains(block, "evolve phase verify build") {
		t.Errorf("block must instruct the self-check command; got:\n%s", block)
	}
	// Cache-safety: the invariant block must NOT embed an absolute path.
	if strings.Contains(block, "/") && strings.Contains(block, "build-report.md") {
		t.Errorf("block must not embed the artifact path (cache-safety); got:\n%s", block)
	}
}

func TestRenderContractBlock_Audit_MentionsVerdictSentinel(t *testing.T) {
	// audit is the verdict-bearing phase, so its block instructs the sentinel.
	block := RenderContractBlock(mustContract(t, "audit"))
	if !strings.Contains(block, "evolve-verdict") {
		t.Errorf("audit block must mention the verdict sentinel; got:\n%s", block)
	}
}

func mustContract(t *testing.T, phase string) Contract {
	t.Helper()
	c, ok := For(phase)
	if !ok {
		t.Fatalf("no contract for %q", phase)
	}
	return c
}

func TestRenderContractBlock_Deterministic(t *testing.T) {
	c, _ := For("audit")
	if RenderContractBlock(c) != RenderContractBlock(c) {
		t.Error("RenderContractBlock must be deterministic (prompt-cache safety)")
	}
}

func TestRenderContractBlock_JSON_UsesRequiredKeys(t *testing.T) {
	c, _ := For("advisor")
	block := RenderContractBlock(c)
	if !strings.Contains(block, "plan") {
		t.Errorf("advisor JSON block must name the required key 'plan'; got:\n%s", block)
	}
	if strings.Contains(block, "evolve-verdict") {
		t.Errorf("JSON deliverable has no verdict sentinel; got:\n%s", block)
	}
}

func TestRenderContractFooter_CarriesExactPath(t *testing.T) {
	c, _ := For("build")
	path := "/abs/.evolve/runs/cycle-213/build-report.md"
	footer := RenderContractFooter(c, path)
	if !strings.Contains(footer, path) {
		t.Errorf("footer must carry the exact path %q; got:\n%s", path, footer)
	}
}

package bridge

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// bridge_directives_test.go — runtime operator-directives block: the pre-rendered
// global + per-loop guidance injected at the same CLI-agnostic seam as
// injectRulesPrefix, sitting just below Correction and above Rules.

func TestInjectOperatorDirectives(t *testing.T) {
	// Empty = identity (off path byte-identical).
	if got := injectOperatorDirectives("BODY", ""); got != "BODY" {
		t.Errorf("empty directives must pass through, got %q", got)
	}
	// Non-empty prepends the (already-rendered) block above the body.
	block := "## Operator Directives\n\nBe env-agnostic."
	got := injectOperatorDirectives("BODY", block)
	if !strings.HasPrefix(got, block) {
		t.Errorf("rendered block must lead: %q", got)
	}
	if !strings.HasSuffix(got, "BODY") || !strings.Contains(got, "Be env-agnostic.") {
		t.Errorf("block/body not assembled: %q", got)
	}
}

// Order: Correction (outermost) > Operator Directives > Rules > Body, mirroring
// Adapter.Launch's assembly seam.
func TestOperatorDirectivesComposeOrder(t *testing.T) {
	withRules := injectRulesPrefix("BODY", "RULE TEXT")
	withDirectives := injectOperatorDirectives(withRules, "## Operator Directives\n\nDIR")
	composed := injectCorrectionPrefix(withDirectives, "CORR")

	corr := strings.Index(composed, "## Correction")
	dir := strings.Index(composed, "## Operator Directives")
	rules := strings.Index(composed, "## Rules")
	body := strings.Index(composed, "BODY")
	if !(corr == 0 && corr < dir && dir < rules && rules < body) {
		t.Fatalf("order must be correction<directives<rules<body; got corr=%d dir=%d rules=%d body=%d\n%s", corr, dir, rules, body, composed)
	}
}

// TestLaunch_InjectsOperatorDirectives proves the REAL Adapter.Launch path injects
// the block when the BridgeRequest carries OperatorDirectives.
func TestLaunch_InjectsOperatorDirectives(t *testing.T) {
	fe := &fakeEngine{}
	_, err := withEngine(fe).Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p", Prompt: "PERSONA-BODY",
		Workspace: t.TempDir(), ArtifactPath: "/a.md", Agent: "build",
		OperatorDirectives: "## Operator Directives\n\nAlways env-agnostic.",
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	got := fe.gotReq.Prompt
	di := strings.Index(got, "## Operator Directives")
	bi := strings.Index(got, "PERSONA-BODY")
	if di < 0 || !strings.Contains(got, "Always env-agnostic.") {
		t.Errorf("operator directives missing from launched prompt:\n%s", truncate(got, 300))
	}
	if di >= bi {
		t.Errorf("directives must precede the body; dir=%d body=%d", di, bi)
	}
}

// TestLaunch_NoOperatorDirectives_WhenEmpty — default path: no directives ⇒ no
// block (byte-identical to pre-feature launches).
func TestLaunch_NoOperatorDirectives_WhenEmpty(t *testing.T) {
	fe := &fakeEngine{}
	_, err := withEngine(fe).Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p", Prompt: "BODY",
		Workspace: t.TempDir(), ArtifactPath: "/a.md", Agent: "build",
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if strings.Contains(fe.gotReq.Prompt, "## Operator Directives") {
		t.Errorf("empty directives must produce no block:\n%s", truncate(fe.gotReq.Prompt, 300))
	}
}

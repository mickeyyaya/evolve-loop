package bridge

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/promptboundary"
)

func TestInjectCorrection(t *testing.T) {
	if got := injectCorrection("BODY", ""); got != "BODY" {
		t.Errorf("empty directive must pass through, got %q", got)
	}
	got := injectCorrection("BODY"+promptboundary.Boundary+"- cycle: 1\n", "fix the Verdict section")
	want := "BODY" + promptboundary.Boundary + "### Correction\n\nfix the Verdict section\n\n### Cycle facts\n- cycle: 1\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestLaunch_InjectsCorrectionBlock(t *testing.T) {
	fe := &fakeEngine{}
	_, err := withEngine(fe).Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p", Prompt: "PERSONA-BODY",
		Workspace: t.TempDir(), ArtifactPath: "/a.md", Agent: "build",
		CorrectionDirective: "rewrite the Verdict section per the contract",
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	got := fe.gotReq.Prompt
	if strings.HasPrefix(got, "### Correction") || strings.HasPrefix(got, "## Correction") {
		t.Errorf("correction block must not lead the materialized prompt:\n%s", truncate(got, 300))
	}
	if !strings.Contains(got, "rewrite the Verdict section per the contract") {
		t.Errorf("directive text missing from launched prompt:\n%s", truncate(got, 300))
	}
	if ci, bi := strings.Index(got, "### Correction"), strings.Index(got, "PERSONA-BODY"); ci <= bi {
		t.Errorf("correction must follow the body; corr=%d body=%d", ci, bi)
	}
}

func TestLaunch_NoCorrectionBlock_WhenDirectiveEmpty(t *testing.T) {
	fe := &fakeEngine{}
	_, err := withEngine(fe).Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p", Prompt: "BODY",
		Workspace: t.TempDir(), ArtifactPath: "/a.md", Agent: "build",
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if strings.Contains(fe.gotReq.Prompt, "Correction") {
		t.Errorf("empty directive must produce no correction block:\n%s", truncate(fe.gotReq.Prompt, 300))
	}
}

func composeFor(t *testing.T, req core.BridgeRequest) string {
	t.Helper()
	req.CLI, req.Profile, req.Workspace, req.ArtifactPath, req.Agent = "claude-tmux", "/p", t.TempDir(), "/a.md", "build"
	out, err := New().ComposePrompt(req)
	if err != nil {
		t.Fatalf("ComposePrompt: %v", err)
	}
	return out.Prompt
}

func TestComposePrompt_CorrectionBelowBoundary(t *testing.T) {
	body := "PERSONA" + promptboundary.Boundary + "- cycle: 1\n"
	a := composeFor(t, core.BridgeRequest{Prompt: body, CorrectionDirective: "fix A"})
	b := composeFor(t, core.BridgeRequest{Prompt: body, CorrectionDirective: "fix B"})
	none := composeFor(t, core.BridgeRequest{Prompt: body})
	if promptboundary.StaticPrefix(a) != promptboundary.StaticPrefix(b) || promptboundary.StaticPrefix(a) != promptboundary.StaticPrefix(none) {
		t.Fatalf("a Correction changed the static prefix:\n%q\n%q", promptboundary.StaticPrefix(a), promptboundary.StaticPrefix(b))
	}
	if !strings.Contains(a, promptboundary.Boundary+"### Correction\n\nfix A\n\n### Cycle facts\n- cycle: 1\n") {
		t.Errorf("Correction block not directly under the boundary:\n%s", a)
	}
}

func TestComposePrompt_CorrectionWithoutBoundaryKeepsContractTailLast(t *testing.T) {
	got := composeFor(t, core.BridgeRequest{Prompt: "PERSONA", CorrectionDirective: "fix it"})
	if n := strings.Count(got, promptboundary.Boundary); n != 1 {
		t.Fatalf("boundary count=%d want 1:\n%s", n, got)
	}
	if !strings.HasSuffix(strings.TrimSpace(got), "</deliverable-contract>") {
		t.Errorf("the deliverable-contract tail must stay last:\n%s", truncate(got[len(got)-300:], 300))
	}
	if ci, ti := strings.Index(got, "fix it"), strings.Index(got, "<deliverable-contract"); ci > ti {
		t.Errorf("Correction must precede the tail; corr=%d tail=%d", ci, ti)
	}
}

func TestComposePrompt_EmptyCorrectionLeavesBytesUnchanged(t *testing.T) {
	body := "PERSONA" + promptboundary.Boundary + "- cycle: 1\n"
	got := composeFor(t, core.BridgeRequest{Prompt: body})
	if strings.Contains(got, "Correction") || strings.Count(got, "### Cycle facts") != 0 {
		t.Errorf("empty directive must add nothing:\n%s", got)
	}
}

func TestComposePrompt_OperatorDirectivesStayAboveBoundary(t *testing.T) {
	body := "PERSONA" + promptboundary.Boundary + "- cycle: 1\n"
	got := composeFor(t, core.BridgeRequest{Prompt: body, OperatorDirectives: "## Operator Directives\n\nDIR"})
	if !strings.Contains(promptboundary.StaticPrefix(got), "## Operator Directives") {
		t.Errorf("directives must stay in the static prefix:\n%s", got)
	}
}

// Tests for the scout phase. Drives the phase with a fake core.Bridge
// that captures the BridgeRequest and writes a scripted scout-report.md
// to the configured artifact path.
package scout

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/prompts"
	"github.com/mickeyyaya/evolveloop/go/test/fixtures"
)

type fakeBridge struct {
	resp          core.BridgeResponse
	err           error
	writeArtifact string

	gotReq core.BridgeRequest
}

func (f *fakeBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	f.gotReq = req
	if f.writeArtifact != "" && req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte(f.writeArtifact), 0o644)
		f.resp.Stdout = f.writeArtifact
	}
	return f.resp, f.err
}

func (f *fakeBridge) Probe(ctx context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func fakePromptsFS(body string) *prompts.Loader {
	return prompts.NewFromFS(fstest.MapFS{
		"agents/evolve-scout.md": &fstest.MapFile{
			Data: []byte("---\nname: evolve-scout\n---\n" + body),
		},
	})
}

func TestRun_HappyPath_PASSWithBacklog(t *testing.T) {
	ws := t.TempDir()
	body := `# Scout Report

## Gap Analysis
| Area | Gap | Severity |
| --- | --- | --- |
| auth | no rate limit | high |

## Proposed Tasks
1. Add login rate limit
2. Add captcha fallback
3. Audit log redaction

## Handoff JSON
` + "```json\n{\"top_n\":[{\"id\":\"a\",\"action\":\"rate-limit\"}]}\n```\n"
	fb := &fakeBridge{writeArtifact: body, resp: core.BridgeResponse{CostUSD: 0.42}}
	clock := fixtures.FixedClock(time.Unix(1_700_000_000, 0), 120*time.Millisecond)
	phase := New(Config{
		Bridge:  fb,
		Prompts: fakePromptsFS("# Scout body"),
		NowFn:   clock,
	})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle:       7,
		ProjectRoot: "/tmp/proj",
		Workspace:   ws,
		GoalHash:    "deadbeef",
		Context:     map[string]string{"strategy": "incremental"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	if resp.Phase != "scout" {
		t.Errorf("Phase=%q, want scout", resp.Phase)
	}
	if resp.NextPhase != "triage" {
		t.Errorf("NextPhase=%q, want triage", resp.NextPhase)
	}
	if resp.CostUSD != 0.42 {
		t.Errorf("CostUSD=%v, want 0.42", resp.CostUSD)
	}
	if resp.DurationMS != 120 {
		t.Errorf("DurationMS=%d, want 120", resp.DurationMS)
	}
	if fb.gotReq.ArtifactPath != filepath.Join(ws, "scout-report.md") {
		t.Errorf("ArtifactPath=%q", fb.gotReq.ArtifactPath)
	}
	wantProfile := filepath.Join("/tmp/proj", ".evolve", "profiles", "scout.json")
	if fb.gotReq.Profile != wantProfile {
		t.Errorf("Profile=%q, want %q", fb.gotReq.Profile, wantProfile)
	}
	if !strings.Contains(fb.gotReq.Prompt, "# Scout body") {
		t.Errorf("Prompt missing agent body")
	}
	if !strings.Contains(fb.gotReq.Prompt, "strategy: incremental") {
		t.Errorf("Prompt missing strategy context")
	}
	if !strings.Contains(fb.gotReq.Prompt, "cycle: 7") {
		t.Errorf("Prompt missing cycle")
	}
}

func TestRun_NoProposedTasks_FAIL(t *testing.T) {
	// Report missing Proposed Tasks → cannot hand off → FAIL.
	body := "# Scout Report\n\nNothing to do.\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
}

func TestRun_ConvergenceConfirmation_SKIPPED(t *testing.T) {
	// "Convergence" + "nothing to do" → SKIPPED (cycle has nothing actionable).
	body := "# Scout Report\nMode: convergence-confirmation. nothingToDoCount=2.\n## Proposed Tasks\n_none_ — converged.\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle:       3,
		ProjectRoot: "/p",
		Workspace:   t.TempDir(),
		Context:     map[string]string{"strategy": "convergence-confirmation"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictSKIPPED {
		t.Errorf("Verdict=%q, want SKIPPED (convergence)", resp.Verdict)
	}
}

func TestRun_EmptyArtifact_FAIL(t *testing.T) {
	fb := &fakeBridge{writeArtifact: ""}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
}

func TestRun_BridgeError_PropagatesAsFAIL(t *testing.T) {
	bridgeErr := errors.New("bridge: launch failed")
	fb := &fakeBridge{err: bridgeErr}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
	})
	if !errors.Is(err, bridgeErr) {
		t.Errorf("err=%v, want wraps bridgeErr", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
}

func TestRun_MissingBridge_ReturnsError(t *testing.T) {
	phase := New(Config{Prompts: fakePromptsFS("body")})
	_, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1})
	if err == nil || !strings.Contains(err.Error(), "bridge required") {
		t.Fatalf("err=%v, want bridge-required", err)
	}
}

func TestRun_MissingPrompts_ReturnsError(t *testing.T) {
	phase := New(Config{Bridge: &fakeBridge{}})
	_, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1})
	if err == nil || !strings.Contains(err.Error(), "prompts loader required") {
		t.Fatalf("err=%v, want prompts-required", err)
	}
}

func TestRun_AgentLoadFails_ReturnsError(t *testing.T) {
	phase := New(Config{Bridge: &fakeBridge{}, Prompts: prompts.NewFromFS(fstest.MapFS{})})
	_, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1})
	if err == nil {
		t.Fatal("err=nil")
	}
}

func TestRun_EnvOverridesCLIAndModel(t *testing.T) {
	fb := &fakeBridge{writeArtifact: "# Scout\n## Proposed Tasks\n1. x\n"}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("b")})
	_, _ = phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
		Env: map[string]string{"EVOLVE_CLI": "gemini", "EVOLVE_SCOUT_MODEL": "gemini-2.5-pro"},
	})
	if fb.gotReq.CLI != "gemini" {
		t.Errorf("CLI=%q, want gemini", fb.gotReq.CLI)
	}
	if fb.gotReq.Model != "gemini-2.5-pro" {
		t.Errorf("Model=%q, want gemini-2.5-pro", fb.gotReq.Model)
	}
}

func TestName(t *testing.T) {
	p := New(Config{})
	if p.Name() != "scout" {
		t.Errorf("Name=%q, want scout", p.Name())
	}
}

// --- v12.1 Capability 1: phaseflags wiring tests ---

func writeScoutProfile(t *testing.T, contents string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scout.json"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return root
}

// TestComposePrompt_InjectsGoalTextFromContext (cycle-108 bug #3): the
// dispatcher routes --goal-text into Context["goal"]; Scout's prompt
// must surface it so Scout treats the operator's goal as a constraint
// when choosing between backlog work and the meta-goal.
func TestComposePrompt_InjectsGoalTextFromContext(t *testing.T) {
	h := hooks{}
	req := core.PhaseRequest{
		Cycle:       108,
		GoalHash:    "abc",
		ProjectRoot: "/p",
		Workspace:   "/p/.evolve/runs/cycle-108",
		Context: map[string]string{
			"strategy": "ultrathink",
			"goal":     "review the pipeline for self-healing",
		},
	}
	got := h.ComposePrompt("BODY", req)
	if !strings.Contains(got, "- goal: review the pipeline for self-healing") {
		t.Errorf("expected goal text line in scout prompt, got: %q", got)
	}
	if !strings.Contains(got, "- strategy: ultrathink") {
		t.Errorf("strategy still expected: %q", got)
	}
}

func TestComposePrompt_OmitsGoalLineWhenContextEmpty(t *testing.T) {
	h := hooks{}
	req := core.PhaseRequest{Cycle: 1, GoalHash: "x", ProjectRoot: "/p", Workspace: "/w"}
	got := h.ComposePrompt("BODY", req)
	if strings.Contains(got, "- goal:") {
		t.Errorf("no goal in Context should omit the line; got: %q", got)
	}
}

// TestComposePrompt_InjectsChallengeTokenFromRequest (cycle-135 lesson):
// the auditor mandates `<!-- challenge-token: <value> -->` on line 2 of
// every phase report. PR 5 centralized the contract in agent-templates.md;
// PR 6 plumbs the actual token VALUE into scout's prompt via the
// PhaseRequest.ChallengeToken field already on the struct (ports.go:148).
// Without this fix, scout has to mint its own token or fall back to a
// placeholder — cycle 135 audit C1: scout used `59576594e2e8d5c3` instead
// of the actual `5b96ecb69a0c848f` from challenge-token.txt.
func TestComposePrompt_InjectsChallengeTokenFromRequest(t *testing.T) {
	h := hooks{}
	req := core.PhaseRequest{
		Cycle:       135,
		GoalHash:    "abc",
		ProjectRoot: "/p",
		Workspace:   "/p/.evolve/runs/cycle-135",
		Context: map[string]string{
			"challengeToken": "5b96ecb69a0c848f",
		},
	}
	got := h.ComposePrompt("BODY", req)
	if !strings.Contains(got, "- challenge_token: 5b96ecb69a0c848f") {
		t.Errorf("expected challenge_token line in scout prompt, got: %q", got)
	}
}

// TestComposePrompt_OmitsChallengeTokenLineWhenEmpty pins the symmetric
// case: when the orchestrator hasn't minted a token yet, scout's prompt
// stays clean (the agent will then read challenge-token.txt or FAIL
// loudly per agent-templates.md PR 5 contract — no placeholder mint).
func TestComposePrompt_OmitsChallengeTokenLineWhenEmpty(t *testing.T) {
	h := hooks{}
	req := core.PhaseRequest{Cycle: 1, GoalHash: "x", ProjectRoot: "/p", Workspace: "/w"}
	got := h.ComposePrompt("BODY", req)
	if strings.Contains(got, "challenge_token:") {
		t.Errorf("expected no challenge_token line when empty, got: %q", got)
	}
}

// TestClassify_SelectedTasksHeading — cycle-192: the scout template's backlog
// heading is "## Selected Tasks" with "### Task N:" subheadings; the classifier
// must accept it (and the legacy "## Proposed Tasks" + list-item shape) or every
// current scout report false-FAILs.
func TestClassify_SelectedTasksHeading(t *testing.T) {
	current := "# Scout Report\n\n## Selected Tasks\n\n### Task 1: `acs-fix` (HIGH, M)\nbody\n"
	if got := classify(current, "balanced"); got != core.VerdictPASS {
		t.Errorf("## Selected Tasks + ### Task: classify=%q, want PASS", got)
	}
	legacy := "# Scout Report\n\n## Proposed Tasks\n1. do the thing\n"
	if got := classify(legacy, "balanced"); got != core.VerdictPASS {
		t.Errorf("legacy ## Proposed Tasks + list: classify=%q, want PASS", got)
	}
	if got := classify("# Scout Report\n\n## Discovery Summary\nnothing actionable\n", "balanced"); got != core.VerdictFAIL {
		t.Errorf("no tasks section: classify=%q, want FAIL", got)
	}
}

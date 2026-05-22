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

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
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

func fixedClock(t time.Time, dur time.Duration) func() time.Time {
	calls := 0
	return func() time.Time {
		defer func() { calls++ }()
		if calls == 0 {
			return t
		}
		return t.Add(dur)
	}
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
	clock := fixedClock(time.Unix(1_700_000_000, 0), 120*time.Millisecond)
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

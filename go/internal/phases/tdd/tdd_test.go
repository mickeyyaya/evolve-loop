// Tests for the tdd phase. The RED phase agent writes failing tests
// before Builder runs. Artifact = test-report.md (the contract) +
// the RED test files in the worktree.
package tdd

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
	gotReq        core.BridgeRequest
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
		"agents/evolve-tdd-engineer.md": &fstest.MapFile{
			Data: []byte("---\nname: evolve-tdd-engineer\n---\n" + body),
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

func TestRun_HappyPath_PASSWithContract(t *testing.T) {
	ws := t.TempDir()
	contract := `# Team Context (RED Contract)

## Goal
Add rate limiter to /login

## Acceptance
- Returns 429 after 5 attempts in 60s
- Resets on success

## RED Tests
- test/rate_limiter_test.go
- test/login_integration_test.go

## Interfaces
- rate.Limiter (token bucket; 5 req / 60s)
`
	fb := &fakeBridge{writeArtifact: contract, resp: core.BridgeResponse{CostUSD: 0.20}}
	clock := fixedClock(time.Unix(1_700_000_000, 0), 100*time.Millisecond)
	phase := New(Config{
		Bridge:  fb,
		Prompts: fakePromptsFS("# TDD body"),
		NowFn:   clock,
	})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle:       9,
		ProjectRoot: "/tmp/proj",
		Workspace:   ws,
		Worktree:    "/tmp/proj/wt",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	if resp.NextPhase != "build" {
		t.Errorf("NextPhase=%q, want build", resp.NextPhase)
	}
	if resp.DurationMS != 100 {
		t.Errorf("DurationMS=%d, want 100", resp.DurationMS)
	}
	if fb.gotReq.ArtifactPath != filepath.Join(ws, "test-report.md") {
		t.Errorf("ArtifactPath=%q", fb.gotReq.ArtifactPath)
	}
	if fb.gotReq.Worktree != "/tmp/proj/wt" {
		t.Errorf("Worktree=%q, want /tmp/proj/wt", fb.gotReq.Worktree)
	}
	wantProfile := filepath.Join("/tmp/proj", ".evolve", "profiles", "tdd-engineer.json")
	if fb.gotReq.Profile != wantProfile {
		t.Errorf("Profile=%q, want %q", fb.gotReq.Profile, wantProfile)
	}
}

func TestRun_DisabledByEnv_SKIPPED(t *testing.T) {
	// EVOLVE_TEST_PHASE_ENABLED=0 short-circuits TDD (fallback to
	// Builder writing own predicates per CLAUDE.md env table).
	fb := &fakeBridge{}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
		Env: map[string]string{"EVOLVE_TEST_PHASE_ENABLED": "0"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictSKIPPED {
		t.Errorf("Verdict=%q, want SKIPPED", resp.Verdict)
	}
	if resp.NextPhase != "build" {
		t.Errorf("NextPhase=%q, want build", resp.NextPhase)
	}
}

func TestRun_NoAcceptance_FAIL(t *testing.T) {
	body := "# Team Context\n\n## Goal\nx\n\n## RED Tests\n- a_test.go\n"
	// Missing "## Acceptance" → contract incomplete → FAIL.
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
}

func TestRun_NoREDTests_FAIL(t *testing.T) {
	body := "# Team Context\n\n## Goal\nx\n\n## Acceptance\n- y\n"
	// Missing "## RED Tests" → contract incomplete → FAIL.
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
}

func TestRun_EmptyArtifact_FAIL(t *testing.T) {
	fb := &fakeBridge{writeArtifact: ""}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
}

func TestRun_BridgeError_FAIL(t *testing.T) {
	bridgeErr := errors.New("bridge fail")
	fb := &fakeBridge{err: bridgeErr}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
	})
	if !errors.Is(err, bridgeErr) {
		t.Errorf("err=%v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
}

func TestRun_MissingBridge_ReturnsError(t *testing.T) {
	phase := New(Config{Prompts: fakePromptsFS("body")})
	_, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1})
	if err == nil || !strings.Contains(err.Error(), "bridge required") {
		t.Fatalf("err=%v", err)
	}
}

func TestRun_MissingPrompts_ReturnsError(t *testing.T) {
	phase := New(Config{Bridge: &fakeBridge{}})
	_, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1})
	if err == nil || !strings.Contains(err.Error(), "prompts loader required") {
		t.Fatalf("err=%v", err)
	}
}

func TestRun_AgentLoadFails_ReturnsError(t *testing.T) {
	phase := New(Config{Bridge: &fakeBridge{}, Prompts: prompts.NewFromFS(fstest.MapFS{})})
	_, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1})
	if err == nil {
		t.Fatal("err=nil")
	}
}

func TestName(t *testing.T) {
	p := New(Config{})
	if p.Name() != "tdd" {
		t.Errorf("Name=%q, want tdd", p.Name())
	}
}

// --- v12.1 Capability 1: phaseflags wiring tests ---

func writeTddProfile(t *testing.T, contents string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tdd-engineer.json"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return root
}

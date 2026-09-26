package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type scriptedBridge struct {
	responses     map[string]scriptedResp
	defaultRespFn func() scriptedResp
	calls         []string
}

type scriptedResp struct {
	resp core.BridgeResponse
	err  error
}

func (s *scriptedBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	s.calls = append(s.calls, req.CLI)
	if r, ok := s.responses[req.CLI]; ok {
		if req.ArtifactPath != "" && r.err == nil {
			_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
			_ = os.WriteFile(req.ArtifactPath, []byte("ok-from-"+req.CLI), 0o644)
			r.resp.Stdout = "ok-from-" + req.CLI
		}
		return r.resp, r.err
	}
	if s.defaultRespFn != nil {
		return s.defaultRespFn().resp, s.defaultRespFn().err
	}
	if req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte("default-ok"), 0o644)
	}
	return core.BridgeResponse{Stdout: "default-ok"}, nil
}

func (s *scriptedBridge) Probe(_ context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func writeFallbackProfile(t *testing.T, agentName, primaryCLI string, fallback []string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	fb := ""
	if len(fallback) > 0 {
		fb = `, "cli_fallback": ["` + strings.Join(fallback, `","`) + `"]`
	}
	body := `{"name":"` + agentName + `","cli":"` + primaryCLI + `","model_tier_default":"sonnet"` + fb + `}`
	profileBase := strings.TrimPrefix(agentName, "evolve-")
	if err := os.WriteFile(filepath.Join(dir, profileBase+".json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return root
}

func TestRun_FallbackOnBootTimeout_PrimaryFailsSecondarySucceeds(t *testing.T) {
	hooks := &fakeHooks{
		phase: "auditor", agent: "evolve-auditor", model: "sonnet",
		prompt: "x", verdict: core.VerdictPASS, nextPhase: "ship",
	}
	sb := &scriptedBridge{
		responses: map[string]scriptedResp{
			"codex-tmux": {
				resp: core.BridgeResponse{ExitCode: 80, Stderr: "REPL boot timeout"},
				err:  errors.New("bridge: launch exit=80"),
			},
			"claude-tmux": {}, // empty = success
		},
	}
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	r := New(Options{
		Hooks:   hooks,
		Bridge:  sb,
		Prompts: fakePromptsFS("evolve-auditor", "x"),
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root,
		Workspace:   t.TempDir(),
	})
	if err != nil {
		t.Fatalf("expected fallback to succeed; got err=%v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("verdict=%s, want PASS (fallback succeeded)", resp.Verdict)
	}
	if len(sb.calls) != 2 {
		t.Fatalf("expected 2 bridge.Launch calls (primary + fallback); got %d: %v", len(sb.calls), sb.calls)
	}
	if sb.calls[0] != "codex-tmux" || sb.calls[1] != "claude-tmux" {
		t.Errorf("dispatch order = %v, want [codex-tmux claude-tmux]", sb.calls)
	}
}

func TestRun_NoFallbackOnNonTriggerExit(t *testing.T) {
	hooks := &fakeHooks{phase: "auditor", agent: "evolve-auditor", model: "sonnet", prompt: "x"}
	sb := &scriptedBridge{
		responses: map[string]scriptedResp{
			"codex-tmux": {
				resp: core.BridgeResponse{ExitCode: 2, Stderr: "safety gate refused"},
				err:  errors.New("bridge: launch exit=2"),
			},
			"claude-tmux": {},
		},
	}
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	r := New(Options{
		Hooks:   hooks,
		Bridge:  sb,
		Prompts: fakePromptsFS("evolve-auditor", "x"),
	})

	_, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err == nil {
		t.Fatal("expected primary's non-trigger error to surface, got nil")
	}
	if len(sb.calls) != 1 {
		t.Errorf("expected ONLY the primary to be called (1 launch); got %d: %v", len(sb.calls), sb.calls)
	}
	if sb.calls[0] != "codex-tmux" {
		t.Errorf("primary CLI was %q, want codex-tmux", sb.calls[0])
	}
}

func TestRun_ChainExhausted_LastErrorSurfaces(t *testing.T) {
	hooks := &fakeHooks{phase: "auditor", agent: "evolve-auditor", model: "sonnet", prompt: "x"}
	sb := &scriptedBridge{
		responses: map[string]scriptedResp{
			"codex-tmux": {
				resp: core.BridgeResponse{ExitCode: 80}, err: errors.New("bridge: launch exit=80"),
			},
			"claude-tmux": {
				resp: core.BridgeResponse{ExitCode: 127}, err: errors.New("bridge: launch exit=127"),
			},
		},
	}
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	r := New(Options{
		Hooks:   hooks,
		Bridge:  sb,
		Prompts: fakePromptsFS("evolve-auditor", "x"),
	})

	_, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err == nil {
		t.Fatal("expected exhausted-chain error, got nil")
	}
	if !strings.Contains(err.Error(), "exit=127") {
		t.Errorf("error should surface LAST attempt's exit code (127); got %v", err)
	}
	if len(sb.calls) != 2 {
		t.Errorf("expected both candidates tried; got %d: %v", len(sb.calls), sb.calls)
	}
}

// The profile omits cli_fallback_on_exit, so the default trigger list is what fires.
func TestRun_FallbackOnArtifactTimeout_DefaultTriggerListIncludes81(t *testing.T) {
	hooks := &fakeHooks{
		phase: "tdd", agent: "evolve-tdd-engineer", model: "sonnet",
		prompt: "x", verdict: core.VerdictPASS, nextPhase: "build",
	}
	sb := &scriptedBridge{
		responses: map[string]scriptedResp{
			"codex-tmux": {
				resp: core.BridgeResponse{ExitCode: 81, Stderr: "bridge artifact timeout"},
				err:  errors.New("bridge: launch exit=81: core: bridge artifact timeout"),
			},
			"claude-tmux": {}, // success
		},
	}
	root := writeFallbackProfile(t, "evolve-tdd-engineer", "codex-tmux", []string{"claude-tmux"})
	r := New(Options{
		Hooks:    hooks,
		Bridge:   sb,
		Prompts:  fakePromptsFS("evolve-tdd-engineer", "x"),
		VerifyFn: alwaysOKVerify,
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root,
		Workspace:   t.TempDir(),
	})
	if err != nil {
		t.Fatalf("expected fallback on exit=81 to succeed; got err=%v\n"+
			"This is the cycle-122 regression: WS-G's default trigger\n"+
			"list MUST include WS-B's ExitArtifactTimeout (81).", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("verdict=%s, want PASS (fallback succeeded)", resp.Verdict)
	}
	if len(sb.calls) != 2 {
		t.Fatalf("expected 2 bridge.Launch calls (primary + fallback); got %d: %v", len(sb.calls), sb.calls)
	}
	if sb.calls[0] != "codex-tmux" || sb.calls[1] != "claude-tmux" {
		t.Errorf("dispatch order = %v, want [codex-tmux claude-tmux]", sb.calls)
	}
}

func TestRun_FallbackOnGNUTimeout_124(t *testing.T) {
	hooks := &fakeHooks{
		phase: "tdd", agent: "evolve-tdd-engineer", model: "sonnet",
		prompt: "x", verdict: core.VerdictPASS, nextPhase: "build",
	}
	sb := &scriptedBridge{
		responses: map[string]scriptedResp{
			"codex-tmux": {
				resp: core.BridgeResponse{ExitCode: 124, Stderr: "timeout"},
				err:  errors.New("bridge: launch exit=124"),
			},
			"claude-tmux": {},
		},
	}
	root := writeFallbackProfile(t, "evolve-tdd-engineer", "codex-tmux", []string{"claude-tmux"})
	r := New(Options{
		Hooks:    hooks,
		Bridge:   sb,
		Prompts:  fakePromptsFS("evolve-tdd-engineer", "x"),
		VerifyFn: alwaysOKVerify,
	})
	resp, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root, Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("expected fallback on exit=124 (GNU timeout); got err=%v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("verdict=%s, want PASS", resp.Verdict)
	}
	if len(sb.calls) != 2 || sb.calls[1] != "claude-tmux" {
		t.Errorf("expected fallback to claude-tmux on exit=124; calls=%v", sb.calls)
	}
}

func TestRun_NoFallback_ByteIdentical(t *testing.T) {
	hooks := &fakeHooks{
		phase: "scout", agent: "evolve-scout", model: "sonnet",
		prompt: "x", verdict: core.VerdictPASS,
	}
	sb := &scriptedBridge{} // empty → default success for any cli
	root := writeFallbackProfile(t, "evolve-scout", "claude-tmux", nil)
	r := New(Options{
		Hooks:    hooks,
		Bridge:   sb,
		Prompts:  fakePromptsFS("evolve-scout", "x"),
		VerifyFn: alwaysOKVerify,
	})
	resp, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("no-fallback path err=%v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("verdict=%s, want PASS", resp.Verdict)
	}
	if len(sb.calls) != 1 {
		t.Errorf("expected exactly 1 launch (no fallback); got %d: %v", len(sb.calls), sb.calls)
	}
}

func TestRun_FallbackOnArtifactTimeout_CarriesVerdictCostDuration(t *testing.T) {
	hooks := &fakeHooks{
		phase: "build", agent: "evolve-builder", model: "sonnet",
		prompt: "x", verdict: core.VerdictPASS, nextPhase: "audit",
	}
	sb := &scriptedBridge{
		responses: map[string]scriptedResp{
			"codex-tmux": {
				resp: core.BridgeResponse{ExitCode: 81, Stderr: "artifact timeout"},
				err:  fmt.Errorf("bridge: launch exit=81: %w", core.ErrArtifactTimeout),
			},
			"claude-tmux": {
				resp: core.BridgeResponse{CostUSD: 0.37, BootMS: 1200},
			},
		},
	}
	root := writeFallbackProfile(t, "evolve-builder", "codex-tmux", []string{"claude-tmux"})
	base := time.Date(2026, 6, 9, 14, 0, 0, 0, time.UTC)
	tick := 0
	r := New(Options{
		Hooks:    hooks,
		Bridge:   sb,
		Prompts:  fakePromptsFS("evolve-builder", "x"),
		VerifyFn: alwaysOKVerify,
		NowFn: func() time.Time {
			tick++
			return base.Add(time.Duration(tick) * time.Second)
		},
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("fallback success must return nil error (the timeout was the PRIMARY's, not the phase's); got %v", err)
	}
	if got := []string{"codex-tmux", "claude-tmux"}; len(sb.calls) != 2 || sb.calls[0] != got[0] || sb.calls[1] != got[1] {
		t.Fatalf("dispatch chain=%v, want %v", sb.calls, got)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("verdict=%s, want PASS (the fallback attempt's own Classify verdict)", resp.Verdict)
	}
	if resp.CostUSD != 0.37 {
		t.Errorf("CostUSD=%v, want 0.37 (the final attempt's cost must survive into the response)", resp.CostUSD)
	}
	if resp.BootMS != 1200 {
		t.Errorf("BootMS=%v, want 1200", resp.BootMS)
	}
	if resp.DurationMS <= 0 {
		t.Errorf("DurationMS=%v, want >0", resp.DurationMS)
	}
}

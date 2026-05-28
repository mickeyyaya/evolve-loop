package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Workstream G1 integration tests for the bridge-dispatch fallback loop.
//
// A scripted bridge produces one response per CLI it's asked to launch
// against, so the test can pin the EXACT chain behavior: which CLI was
// dispatched, in what order, and which one's response the runner returned.

// scriptedBridge maps cli → BridgeResponse and records the launch order.
// Defaults are exit=0 (success) when a cli isn't scripted, so non-fallback
// paths stay simple. Set the cli's entry to a scripted error+exit to fire
// fallback behavior under test.
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
		// Write the scripted artifact so the runner's downstream Read +
		// Classify path doesn't trip on a missing file.
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

// writeFallbackProfile drops a profile JSON with cli + cli_fallback fields
// into a temp .evolve/profiles dir under a fresh tempDir. The agent name
// follows the runner's strip-"evolve-" convention (profile file is named
// after the agent WITHOUT the prefix). Returns the projectRoot for
// PhaseRequest.ProjectRoot.
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
	// Strip the "evolve-" prefix the way the runner does (AgentPromptName=
	// "evolve-auditor" → profile file "auditor.json").
	profileBase := strings.TrimPrefix(agentName, "evolve-")
	if err := os.WriteFile(filepath.Join(dir, profileBase+".json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return root
}

// TestRun_FallbackOnBootTimeout_PrimaryFailsSecondarySucceeds is the
// canonical cycle-121 fix: primary cli returns exit=80 (REPL boot timeout),
// fallback cli succeeds, runner returns PASS without surfacing the primary
// failure as an error.
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

// TestRun_NoFallbackOnNonTriggerExit pins the SCOPING contract: a non-trigger
// exit (e.g. exit=2 ExitSafetyGate, or a generic non-zero) does NOT advance
// the chain. The primary's error surfaces — a legitimate FAIL never silently
// routes to a different CLI.
func TestRun_NoFallbackOnNonTriggerExit(t *testing.T) {
	hooks := &fakeHooks{phase: "auditor", agent: "evolve-auditor", model: "sonnet", prompt: "x"}
	sb := &scriptedBridge{
		responses: map[string]scriptedResp{
			"codex-tmux": {
				resp: core.BridgeResponse{ExitCode: 2, Stderr: "safety gate refused"},
				err:  errors.New("bridge: launch exit=2"),
			},
			// claude-tmux configured but should NOT be reached
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

// TestRun_ChainExhausted_LastErrorSurfaces pins what happens when ALL
// candidates trip a trigger: the runner returns the last attempt's error,
// not the first. Operator can see the chain in the dispatch log.
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

// TestRun_NoFallback_ByteIdentical pins the opt-out contract: a profile
// without cli_fallback set + no env override behaves exactly like pre-G —
// single launch, single error path. This is the regression guard for the
// 6 cycle-119/120 workstreams that ship with no fallback configured.
func TestRun_NoFallback_ByteIdentical(t *testing.T) {
	hooks := &fakeHooks{
		phase: "scout", agent: "evolve-scout", model: "sonnet",
		prompt: "x", verdict: core.VerdictPASS,
	}
	sb := &scriptedBridge{} // empty → default success for any cli
	root := writeFallbackProfile(t, "evolve-scout", "claude-tmux", nil)
	r := New(Options{
		Hooks:   hooks,
		Bridge:  sb,
		Prompts: fakePromptsFS("evolve-scout", "x"),
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

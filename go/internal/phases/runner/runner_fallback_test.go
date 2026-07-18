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

// TestRun_FallbackOnArtifactTimeout_DefaultTriggerListIncludes81 is the
// cycle-122 cross-workstream contract test (Fix 2 of the cycle-122
// remediation). It pins the WS-B↔WS-G integration that the prior
// session shipped without: WS-B introduced ExitArtifactTimeout (81)
// as the bridge's coarse stall-detection signal; WS-G's fallback chain
// triggers on a list of exit codes. The default trigger list MUST
// include 81 so artifact-timeout failures route to the next CLI
// instead of aborting the cycle.
//
// Without this guarantee, cycle-122's codex-tmux tdd-phase hang
// (which the artifact-timeout caught at exit=81) was not retried on
// any other CLI — see docs/incidents/cycle-122-...md for the full
// failure analysis.
//
// Profile sets cli + cli_fallback but DOES NOT set
// cli_fallback_on_exit, so the default trigger list is exercised.
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
		VerifyFn: alwaysOKVerify, // plumbing test — isolate from the deliverable hard-gate
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

// TestRun_FallbackOnGNUTimeout_124 is the defensive companion to the
// cycle-122 fix: coreutils `timeout(1)` exits 124 when its time limit
// trips. If anything wraps a CLI in `timeout`, that 124 should retry
// on the next CLI rather than abort. Same default-trigger-list contract.
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
		VerifyFn: alwaysOKVerify, // plumbing test — isolate from the deliverable hard-gate
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
		Hooks:    hooks,
		Bridge:   sb,
		Prompts:  fakePromptsFS("evolve-scout", "x"),
		VerifyFn: alwaysOKVerify, // plumbing test — isolate from the deliverable hard-gate
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

// TestRun_FallbackOnArtifactTimeout_CarriesVerdictCostDuration is the
// cycle-262 link at the runner level (ADR-0044 C1 / Slice 1): primary CLI
// exits 81 (artifact timeout — exactly what codex's mid-phase self-upgrade
// produced), the fallback CLI succeeds, and the runner's response must carry
// the FINAL attempt's verdict + cost + boot, a positive duration, and a nil
// error. Baseline-GREEN pin: the dispatch chain already behaves this way (the
// 262 recording loss was downstream, in the orchestrator's abort paths) —
// this test makes the link regression-proof while C1 reshapes the recording.
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
		VerifyFn: alwaysOKVerify, // plumbing test — isolate from the deliverable hard-gate
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

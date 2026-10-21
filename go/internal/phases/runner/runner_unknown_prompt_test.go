package runner

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRun_AnUndeliveredPromptIsNoWallThatTheWalkSurfaces(t *testing.T) {
	hooks := &fakeHooks{phase: "auditor", agent: "evolve-auditor", model: "sonnet", prompt: "x"}
	sb := &scriptedBridge{
		responses: map[string]scriptedResp{
			"codex-tmux":  {resp: core.BridgeResponse{ExitCode: 85, CauseCode: "unknown_prompt"}, err: errors.New("bridge: launch exit=85: unknown interactive prompt")},
			"claude-tmux": {resp: core.BridgeResponse{ExitCode: 81, UsageExhausted: true}, err: errors.New("bridge: launch exit=81: artifact timeout")},
		},
	}
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	r := New(Options{Hooks: hooks, Bridge: sb, Prompts: fakePromptsFS("evolve-auditor", "x")})

	_, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})

	if err == nil || strings.Contains(err.Error(), "exit=85") || !strings.Contains(err.Error(), "exit=81") || len(sb.calls) != 2 {
		t.Fatalf("an undelivered prompt still falls back, but it is no quota wall, so the walk surfaces the last failure; err=%v calls=%v", err, sb.calls)
	}
}

func TestRun_TheErrorCarriesTheCauseOfTheLastAttempt(t *testing.T) {
	hooks := &fakeHooks{phase: "auditor", agent: "evolve-auditor", model: "sonnet", prompt: "x"}
	sb := &scriptedBridge{
		responses: map[string]scriptedResp{
			"codex-tmux": {resp: core.BridgeResponse{ExitCode: 85, CauseCode: "unknown_prompt"}, err: errors.New("bridge: launch exit=85: unknown interactive prompt")},
		},
	}
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", nil)
	r := New(Options{Hooks: hooks, Bridge: sb, Prompts: fakePromptsFS("evolve-auditor", "x")})

	_, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})

	var cause core.LaunchCause
	if !errors.As(err, &cause) || cause.Cause != "unknown_prompt" || !strings.Contains(err.Error(), "exit=85") {
		t.Fatalf("err = %v: the cycle layer reads the typed cause, not the exit text", err)
	}
}

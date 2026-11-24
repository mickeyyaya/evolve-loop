package runner

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

var errGuardStall = errors.New("bridge: launch exit=81: guard ship denied git push origin main before the launch stalled")

func stallThroughThreeFamilies(t *testing.T, cause string, stallErr error) (*scriptedBridge, error) {
	t.Helper()
	hooks := &fakeHooks{phase: "auditor", agent: "evolve-auditor", model: "sonnet", prompt: "x"}
	stall := scriptedResp{
		resp: core.BridgeResponse{ExitCode: 81, CauseCode: cause},
		err:  stallErr,
	}
	sb := &scriptedBridge{responses: map[string]scriptedResp{"codex-tmux": stall, "claude-tmux": stall, "agy-tmux": stall}}
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux", "agy-tmux"})
	r := New(Options{Hooks: hooks, Bridge: sb, Prompts: fakePromptsFS("evolve-auditor", "x")})
	_, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	return sb, err
}

func TestDispatch_AGuardRefusalEndsTheChainAfterOneAttempt(t *testing.T) {
	t.Parallel()
	sb, err := stallThroughThreeFamilies(t, core.CauseGuardRefused, errGuardStall)
	if len(sb.calls) != 1 || sb.calls[0] != "codex-tmux" {
		t.Fatalf("calls %v, want one attempt", sb.calls)
	}
	var cause core.LaunchCause
	if !errors.As(err, &cause) || cause.Cause != core.CauseGuardRefused || !errors.Is(err, errGuardStall) {
		t.Fatalf("the terminal error carries the cause guard_refused and the stall of the guard: %v", err)
	}
	if errors.Is(err, core.ErrAllFamiliesExhausted) || errors.Is(err, core.ErrTransientBridgeFailure) {
		t.Fatalf("a guard refusal is no quota wall and no transient fault: %v", err)
	}
}

func TestDispatch_AStallWithoutADenialKeepsTheChain(t *testing.T) {
	t.Parallel()
	sb, _ := stallThroughThreeFamilies(t, "artifact_timeout", errors.New("bridge: launch exit=81"))
	if len(sb.calls) != 3 {
		t.Fatalf("calls %v, want all three families", sb.calls)
	}
}

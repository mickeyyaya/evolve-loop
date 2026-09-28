package bridgechain_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func TestWallKeeper_SurfacesTheWallOfAWalledWalkNamingEveryRung(t *testing.T) {
	var keeper bridgechain.WallKeeper
	wall, stall := core.BridgeResponse{ExitCode: 85}, core.BridgeResponse{ExitCode: 81}
	keeper.Observe("codex-tmux@balanced", wall, errors.New("bridge: launch exit=85"))
	keeper.Observe("claude-tmux@balanced", stall, errors.New("bridge: launch exit=81"))

	res, err := keeper.Surface(llmroute.TieredDispatchResult{Walled: true}, stall, errors.New("bridge: launch exit=81"))

	if res.ExitCode != 85 || err == nil || !strings.Contains(err.Error(), "exit=85") || !strings.Contains(err.Error(), "codex-tmux@balanced=85 -> claude-tmux@balanced=81") {
		t.Fatalf("a walled walk surfaces its wall and names every rung: res=%+v err=%v", res, err)
	}
	if res, err := keeper.Surface(llmroute.TieredDispatchResult{}, stall, errors.New("bridge: launch exit=81")); res.ExitCode != 81 || !strings.Contains(err.Error(), "exit=81") {
		t.Fatalf("a walk that is not walled surfaces its last rung: res=%+v err=%v", res, err)
	}
}

func TestWalking_AnExhaustedWalkThatMetAWallSurfacesTheWall(t *testing.T) {
	inner := &scripted{exits: map[string]int{"codex-tmux": 85, "claude-tmux": 81}}
	w := bridgechain.New(inner, fixedPlan([]string{"codex-tmux", "claude-tmux"}, []string{"balanced"}))

	resp, err := w.Launch(context.Background(), core.BridgeRequest{CLI: "codex-tmux", Model: "balanced", Agent: "triage"})

	if resp.ExitCode != 85 || err == nil || !strings.Contains(err.Error(), "claude-tmux@balanced=81") {
		t.Fatalf("the handle's walk defers like the runner's: resp=%+v err=%v", resp, err)
	}
}

func TestWalking_AWallThenARealFailureSurfacesTheFailure(t *testing.T) {
	inner := &scripted{exits: map[string]int{"codex-tmux": 85, "claude-tmux": 3}}
	w := bridgechain.New(inner, fixedPlan([]string{"codex-tmux", "claude-tmux"}, []string{"balanced"}))

	resp, err := w.Launch(context.Background(), core.BridgeRequest{CLI: "codex-tmux", Model: "balanced", Agent: "triage"})

	if resp.ExitCode != 3 || err == nil || !strings.Contains(err.Error(), "exit 3") {
		t.Fatalf("a real failure after a wall stays the verdict: resp=%+v err=%v", resp, err)
	}
}

func TestWallKeeper_ALastRungThatIsTheWallSurfacesUnwrapped(t *testing.T) {
	var keeper bridgechain.WallKeeper
	stall, wall := core.BridgeResponse{ExitCode: 81}, core.BridgeResponse{ExitCode: 85}
	wallErr := errors.New("bridge: launch exit=85")
	keeper.Observe("claude-tmux@balanced", stall, errors.New("bridge: launch exit=81"))
	keeper.Observe("codex-tmux@balanced", wall, wallErr)
	walk := llmroute.TieredDispatchResult{Walled: true}

	if keeper.Surfaces(walk, wall) {
		t.Fatal("a walk whose last rung is the wall has nothing to surface")
	}
	if res, err := keeper.Surface(walk, wall, wallErr); res.ExitCode != 85 || err != wallErr {
		t.Fatalf("the last rung's wall returns as it is: res=%+v err=%v", res, err)
	}
}

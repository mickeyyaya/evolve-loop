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
	wall, stall := core.BridgeResponse{ExitCode: 85}, core.BridgeResponse{ExitCode: 81, UsageExhausted: true}
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
	inner := &scripted{exits: map[string]int{"codex-tmux": 85, "claude-tmux": 81}, exhausted: map[string]bool{"claude-tmux": true}}
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
	stall, wall := core.BridgeResponse{ExitCode: 81, UsageExhausted: true}, core.BridgeResponse{ExitCode: 85}
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

func TestWallKeeper_AStallWithNoExhaustionEvidenceBeforeTheWallIsNotAQuotaPause(t *testing.T) {
	var keeper bridgechain.WallKeeper
	lost, wall := core.BridgeResponse{ExitCode: 81}, core.BridgeResponse{ExitCode: 85}
	wallErr := errors.New("bridge: launch exit=85: escalation report written (pattern=exhausted reason=escalate)")
	keeper.Observe("claude-tmux@deep", lost, errors.New("bridge: launch exit=81: artifact-timeout: cause=pane_lost"))
	keeper.Observe("agy-claude-tmux@deep", wall, wallErr)

	res, err := keeper.Surface(llmroute.TieredDispatchResult{Walled: true}, wall, wallErr)

	if res.ExitCode != 81 || err == nil || !strings.Contains(err.Error(), "cause=pane_lost") {
		t.Fatalf("the rung that failed without exhaustion evidence is the verdict, never the wall: res=%+v err=%v", res, err)
	}
	if !strings.Contains(err.Error(), "claude-tmux@deep=81 -> agy-claude-tmux@deep=85") || !strings.Contains(err.Error(), "no evidence of exhaustion") {
		t.Errorf("the surfaced failure must name every rung and why the walk is not a pause: %v", err)
	}
}

func TestWallKeeper_AStallWithExhaustionEvidenceBeforeTheWallIsStillAQuotaPause(t *testing.T) {
	var keeper bridgechain.WallKeeper
	drained, wall := core.BridgeResponse{ExitCode: 81, UsageExhausted: true}, core.BridgeResponse{ExitCode: 85}
	wallErr := errors.New("bridge: launch exit=85")
	keeper.Observe("claude-tmux@deep", drained, errors.New("bridge: launch exit=81"))
	keeper.Observe("agy-claude-tmux@deep", wall, wallErr)
	walk := llmroute.TieredDispatchResult{Walled: true}

	res, err := keeper.Surface(walk, wall, wallErr)

	if keeper.Surfaces(walk, wall) || res.ExitCode != 85 || err != wallErr {
		t.Fatalf("every rung proven exhausted keeps the wall: res=%+v err=%v", res, err)
	}
}

func TestWalking_TheCycle1853WalkWithAnUnprovenPaneLossSurfacesThePaneLoss(t *testing.T) {
	for _, tc := range []struct {
		name      string
		exhausted bool
		want      int
	}{
		{name: "healthy or unknown usage", exhausted: false, want: 81},
		{name: "exhausted usage", exhausted: true, want: 85},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inner := &scripted{exits: map[string]int{"claude-tmux": 81, "agy-claude-tmux": 85}, exhausted: map[string]bool{"claude-tmux": tc.exhausted}}
			w := bridgechain.New(inner, fixedPlan([]string{"claude-tmux", "agy-claude-tmux"}, []string{"deep"}))

			resp, err := w.Launch(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: "deep", Agent: "build"})

			if resp.ExitCode != tc.want || err == nil {
				t.Fatalf("resp=%+v err=%v, want exit %d", resp, err, tc.want)
			}
		})
	}
}

func TestWallKeeper_ARungThatNeverStartedWorkIsNeutralSoTheRealWallStillPauses(t *testing.T) {
	for _, tc := range []struct {
		name string
		exit int
	}{
		{"agy-claude model label missing (87)", 87},
		{"binary missing (127)", 127},
		{"REPL never booted (80)", 80},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inner := &scripted{exits: map[string]int{"agy-claude-tmux": tc.exit, "claude-tmux": 85}}
			w := bridgechain.New(inner, fixedPlan([]string{"agy-claude-tmux", "claude-tmux"}, []string{"deep"}))

			resp, err := w.Launch(context.Background(), core.BridgeRequest{CLI: "agy-claude-tmux", Model: "deep", Agent: "build"})

			if resp.ExitCode != 85 || err == nil {
				t.Fatalf("resp=%+v err=%v, want the real wall (85): a rung that never started work proves nothing either way", resp, err)
			}
		})
	}
}

func TestWallKeeper_ANonStartAfterTheWallStillSurfacesTheWall(t *testing.T) {
	var keeper bridgechain.WallKeeper
	wall, mismatch := core.BridgeResponse{ExitCode: 85}, core.BridgeResponse{ExitCode: 87}
	keeper.Observe("claude-tmux@deep", wall, errors.New("bridge: launch exit=85"))
	keeper.Observe("agy-claude-tmux@deep", mismatch, errors.New("bridge: launch exit=87"))

	res, err := keeper.Surface(llmroute.TieredDispatchResult{Walled: true}, mismatch, errors.New("bridge: launch exit=87"))

	if res.ExitCode != 85 || err == nil || !strings.Contains(err.Error(), "met a quota wall before its last rung failed") {
		t.Fatalf("res=%+v err=%v, want the wall: a non-start on the last rung is neutral", res, err)
	}
}

func TestWalking_ALoneExhaustedUsageReadWithNoWallIsNoPause(t *testing.T) {
	inner := &scripted{exits: map[string]int{"claude-tmux": 1}, exhausted: map[string]bool{"claude-tmux": true}}
	w := bridgechain.New(inner, fixedPlan([]string{"claude-tmux"}, []string{"deep"}))

	resp, err := w.Launch(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: "deep", Agent: "build"})

	if resp.ExitCode != 1 || err == nil {
		t.Fatalf("resp=%+v err=%v, want the plain exit-1 failure: a pause needs an exit 85 in the walk", resp, err)
	}
}

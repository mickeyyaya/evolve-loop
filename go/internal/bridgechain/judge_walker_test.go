package bridgechain_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestWalking_WithCopiesTheInnerBenchAndLogAndSwapsResolverAndFilter(t *testing.T) {
	inner := &scripted{exits: map[string]int{"codex-tmux": 85}}
	var benched, logs []string
	base := bridgechain.New(inner, fixedPlan([]string{"claude-tmux"}, []string{"deep"}),
		bridgechain.WithBench(func(e bridgechain.Escalation) { benched = append(benched, e.CLI) }),
		bridgechain.WithLog(func(f string, _ ...any) { logs = append(logs, f) }),
		bridgechain.WithCandidateFilter(func(string, string) bool { return false }))
	derived := base.With(fixedPlan([]string{"codex-tmux", "agy-tmux"}, []string{"deep"}), func(cli, _ string) bool { return cli != "claude-tmux" })
	seat, _, err := derived.LaunchSeat(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: "deep", Agent: "handoff-judge"})
	if err != nil {
		t.Fatal(err)
	}
	if seat.CLI != "agy-tmux" || strings.Join(attempts(inner.calls), " ") != "codex-tmux@deep agy-tmux@deep" {
		t.Fatalf("seat=%+v attempts=%v: With must swap the resolver and the filter", seat, attempts(inner.calls))
	}
	if strings.Join(benched, " ") != "codex-tmux" {
		t.Fatalf("benched = %v: With must keep the bench of the base walker", benched)
	}
	if len(logs) == 0 {
		t.Fatal("With must keep the log of the base walker")
	}
	if _, _, err := base.LaunchSeat(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: "deep"}); err == nil {
		t.Fatal("deriving a walker must leave the base walker unchanged")
	}
}

func TestWalking_WithKeepsTheClockOfTheBaseWalker(t *testing.T) {
	fixed := time.Date(2026, 11, 21, 1, 2, 3, 0, time.UTC)
	var starts []time.Time
	base := bridgechain.New(&scripted{exits: map[string]int{"codex-tmux": 85}}, nil,
		bridgechain.WithClock(func() time.Time { return fixed }),
		bridgechain.WithBench(func(e bridgechain.Escalation) { starts = append(starts, e.DispatchStart) }))
	derived := base.With(fixedPlan([]string{"codex-tmux", "agy-tmux"}, []string{"deep"}), nil)
	if _, _, err := derived.LaunchSeat(context.Background(), core.BridgeRequest{CLI: "codex-tmux", Model: "deep"}); err != nil {
		t.Fatal(err)
	}
	if len(starts) != 1 || !starts[0].Equal(fixed) {
		t.Fatalf("dispatch starts = %v, want the base clock %v", starts, fixed)
	}
}

func TestTierOverlayResolver_MapsTheRequestLikeDefaultPlanResolverAndAddsTheTier(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	writeProfile(t, dir, "handoff-judge", map[string]any{"name": "handoff-judge", "cli": "agy-tmux", "model_tier_default": "deep"})
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	router := routerOver(t, dir, policy.Policy{CLIRouting: &block})
	req := core.BridgeRequest{Agent: "handoff-judge", CLI: "agy-tmux", Model: "balanced", ProjectRoot: root, Worktree: root}

	plain := mustPlan(t, bridgechain.DefaultPlanResolver(router), req)
	overlaid := mustPlan(t, bridgechain.TierOverlayResolver(router, "balanced"), req)
	if len(plain.Tiers) == 0 || plain.Tiers[0] != "deep" {
		t.Fatalf("the plain resolver lets the profile default win: tiers = %v", plain.Tiers)
	}
	if len(overlaid.Tiers) == 0 || overlaid.Tiers[0] != "balanced" {
		t.Fatalf("the overlay must set the tier of the plan: tiers = %v", overlaid.Tiers)
	}
	if strings.Join(plain.Candidates, " ") != strings.Join(overlaid.Candidates, " ") || plain.PrimarySource != overlaid.PrimarySource {
		t.Fatalf("the request mapping must be the one of DefaultPlanResolver: %+v vs %+v", plain, overlaid)
	}
}

func TestServedModel_UnknownSeatIsNotOK(t *testing.T) {
	if _, ok := bridgechain.ServedModel("claude-tmux", "no-such-tier"); ok {
		t.Fatal("a tier with no row in the tier map has no served model")
	}
	if _, ok := bridgechain.ServedModel("no-such-driver", "deep"); ok {
		t.Fatal("an unknown driver has no served model")
	}
	if model, ok := bridgechain.ServedModel("claude-tmux", "deep"); !ok || model == "" || model == "deep" {
		t.Fatalf("claude-tmux deep = %q, %v; want the model of the manifest tier map", model, ok)
	}
}

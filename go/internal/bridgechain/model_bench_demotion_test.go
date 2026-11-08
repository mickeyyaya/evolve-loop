package bridgechain_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

var modelBenchClock = func() time.Time { return time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC) }

func benchModel(t *testing.T, root, family, word string, reset time.Duration) {
	t.Helper()
	wall := clihealth.Wall{Pattern: clihealth.UsageProbePattern, Evidence: word + " week window 100% used", Reset: modelBenchClock().Add(reset)}
	if _, err := clihealth.NewStore(root, modelBenchClock).BenchModelUntil(family, word, wall); err != nil {
		t.Fatal(err)
	}
}

func chainWith(tiers ...string) llmroute.Plan {
	return llmroute.Plan{Candidates: []string{"claude-tmux", "agy-claude-tmux", "codex-tmux"}, Tiers: tiers}
}

func TestApplyModelBench_DemotesOnlyTheTiersThatResolveToTheBenchedModel(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchModel(t, root, "claude", "Opus", 5*time.Hour)

	deep := bridgechain.ApplyCLIHealthBench(root, "audit", chainWith("deep"), modelBenchClock, nil)
	top := bridgechain.ApplyCLIHealthBench(root, "audit", chainWith("top"), modelBenchClock, nil)
	balanced := bridgechain.ApplyCLIHealthBench(root, "audit", chainWith("balanced"), modelBenchClock, nil)
	fast := bridgechain.ApplyCLIHealthBench(root, "audit", chainWith("fast"), modelBenchClock, nil)

	for name, out := range map[string]llmroute.Plan{"deep": deep, "top": top} {
		if want := []string{"agy-claude-tmux", "codex-tmux", "claude-tmux"}; !slices.Equal(out.Candidates, want) {
			t.Errorf("%s chain = %v, want %v: claude-tmux resolves to Opus here, so it goes last and agy-claude-tmux (the same model on agy's own pool) leads", name, out.Candidates, want)
		}
	}
	for name, out := range map[string]llmroute.Plan{"balanced": balanced, "fast": fast} {
		if want := []string{"claude-tmux", "agy-claude-tmux", "codex-tmux"}; !slices.Equal(out.Candidates, want) {
			t.Errorf("%s chain = %v, want %v: the Opus bench must leave a tier that resolves to another model alone", name, out.Candidates, want)
		}
	}
}

func TestApplyModelBench_ReadsTheTierFromPlanModelWhenThePlanHasNoTierList(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchModel(t, root, "claude", "Opus", 5*time.Hour)
	plan := llmroute.Plan{Candidates: []string{"claude-tmux", "agy-claude-tmux"}, Model: "deep"}

	out := bridgechain.ApplyCLIHealthBench(root, "audit", plan, modelBenchClock, nil)

	if want := []string{"agy-claude-tmux", "claude-tmux"}; !slices.Equal(out.Candidates, want) {
		t.Errorf("chain = %v, want %v", out.Candidates, want)
	}
}

func TestApplyModelBench_AnUnmatchedScopeWordBenchesNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchModel(t, root, "claude", "Fable", 5*time.Hour)
	want := []string{"claude-tmux", "agy-claude-tmux", "codex-tmux"}

	for _, tier := range []string{"fast", "balanced", "deep", "top"} {
		var logs []string
		out := bridgechain.ApplyCLIHealthBench(root, "audit", chainWith(tier), modelBenchClock, func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) })
		if !slices.Equal(out.Candidates, want) {
			t.Errorf("%s chain = %v, want %v: the manifest's model_match maps no Fable, so the bench is fail-open", tier, out.Candidates, want)
		}
		if strings.Contains(strings.Join(logs, ""), "reordered") {
			t.Errorf("%s: logs say the chain was reordered: %q", tier, logs)
		}
	}
}

func TestApplyModelBench_ABenchPastItsResetDemotesNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchModel(t, root, "claude", "Opus", time.Hour)
	later := func() time.Time { return modelBenchClock().Add(3 * time.Hour) }

	out := bridgechain.ApplyCLIHealthBench(root, "audit", chainWith("deep"), later, nil)

	if want := []string{"claude-tmux", "agy-claude-tmux", "codex-tmux"}; !slices.Equal(out.Candidates, want) {
		t.Errorf("chain = %v, want %v after the Opus week reset", out.Candidates, want)
	}
}

func TestApplyModelBench_LeavesTheFamilyBenchViewAndOtherFamiliesAlone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchModel(t, root, "claude", "Opus", 5*time.Hour)
	var logs []string

	out := bridgechain.ApplyCLIHealthBench(root, "audit", chainWith("deep"), modelBenchClock, func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) })

	if active := clihealth.NewStore(root, modelBenchClock).Active(); len(active) != 0 {
		t.Errorf("Active() = %v: a model bench must not turn into a family bench", active)
	}
	if !slices.Contains(out.Candidates, "claude-tmux") || len(out.Candidates) != 3 {
		t.Errorf("chain = %v: the demoted claude-tmux stays in the chain (advice, not a veto)", out.Candidates)
	}
	joined := strings.Join(logs, "")
	if !strings.Contains(joined, "Opus") || strings.Contains(joined, "ALL candidates benched") {
		t.Errorf("logs = %q, want one reorder line naming Opus and no all-benched warning", logs)
	}
}

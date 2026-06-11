package llmroute

// RED contract for ApplyBench (cycle-283): the dispatch chain must START at a
// healthy CLI when the primary's family is benched, mirroring Probe's
// demote-not-drop reorder. Bench is advice, never a veto: with every family
// benched, the chain runs least-recently-benched first.

import (
	"testing"
	"time"
)

func TestFamilyMapsDriverToBinary(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"codex-tmux":  "codex",
		"codex":       "codex",
		"claude-tmux": "claude",
		"claude-p":    "claude",
		"agy-tmux":    "agy",
		"ollama-tmux": "ollama",
		"unknown-cli": "unknown-cli", // unknown maps to itself
	}
	for cli, want := range cases {
		if got := Family(cli); got != want {
			t.Errorf("Family(%q)=%q, want %q", cli, got, want)
		}
	}
}

func TestApplyBenchDemotesBenchedFamily(t *testing.T) {
	t.Parallel()
	p := Plan{Candidates: []string{"codex-tmux", "claude-tmux"}, Triggers: []int{85}}
	out := ApplyBench(p, map[string]time.Time{"codex": time.Now()})
	want := []string{"claude-tmux", "codex-tmux"}
	for i, w := range want {
		if out.Candidates[i] != w {
			t.Fatalf("candidates=%v, want %v (benched codex demoted, never dropped)", out.Candidates, want)
		}
	}
	if out.Triggers[0] != 85 {
		t.Error("ApplyBench must carry non-Candidates Plan fields through (copy-struct convention)")
	}
}

func TestApplyBenchPreservesOrderAmongHealthy(t *testing.T) {
	t.Parallel()
	p := Plan{Candidates: []string{"codex-tmux", "claude-tmux", "agy-tmux"}}
	out := ApplyBench(p, map[string]time.Time{"codex": time.Now()})
	want := []string{"claude-tmux", "agy-tmux", "codex-tmux"}
	for i, w := range want {
		if out.Candidates[i] != w {
			t.Fatalf("candidates=%v, want %v", out.Candidates, want)
		}
	}
}

func TestApplyBenchAllBenchedLeastRecentFirst(t *testing.T) {
	t.Parallel()
	older := time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	p := Plan{Candidates: []string{"codex-tmux", "claude-tmux"}}
	out := ApplyBench(p, map[string]time.Time{"codex": newer, "claude": older})
	want := []string{"claude-tmux", "codex-tmux"} // least-recently-benched first
	for i, w := range want {
		if out.Candidates[i] != w {
			t.Fatalf("all-benched candidates=%v, want %v (bench is advice, not a veto)", out.Candidates, want)
		}
	}
}

func TestApplyBenchNoOpCases(t *testing.T) {
	t.Parallel()
	single := Plan{Candidates: []string{"codex-tmux"}}
	if out := ApplyBench(single, map[string]time.Time{"codex": time.Now()}); out.Candidates[0] != "codex-tmux" {
		t.Error("single-candidate plan must be untouched")
	}
	p := Plan{Candidates: []string{"codex-tmux", "claude-tmux"}}
	if out := ApplyBench(p, nil); out.Candidates[0] != "codex-tmux" {
		t.Error("nil bench map must be a no-op")
	}
}

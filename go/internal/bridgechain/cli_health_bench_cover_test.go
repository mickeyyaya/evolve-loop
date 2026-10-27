package bridgechain_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func TestApplyCLIHealthBench_ABootBenchedDriverMovesBackWithTheRealClock(t *testing.T) {
	root := t.TempDir()
	store := clihealth.NewStore(root, nil)
	for i := 0; i < clihealth.DefaultBootBenchThreshold; i++ {
		if _, err := store.RecordBootStrike("codex-tmux"); err != nil {
			t.Fatal(err)
		}
	}
	plan := llmroute.Plan{Candidates: []string{"codex-tmux", "claude-tmux"}}
	if out := bridgechain.ApplyCLIHealthBench(root, "build", plan, nil, nil); strings.Join(out.Candidates, " ") != "claude-tmux codex-tmux" {
		t.Fatalf("out = %v, want the boot-benched driver last", out.Candidates)
	}
	var logs []string
	bridgechain.ApplyCLIHealthBench(root, "build", plan, nil, func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) })
	if len(logs) != 1 || !strings.Contains(logs[0], "driver-bench reordered chain") {
		t.Fatalf("logs = %q, want the one driver-bench line", logs)
	}
}

func TestApplyCLIHealthBench_EveryCandidateBenchedKeepsTheOrderAndWarns(t *testing.T) {
	root := t.TempDir()
	now := func() time.Time { return time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC) }
	store := clihealth.NewStore(root, now)
	for _, family := range []string{"codex", "claude"} {
		if _, err := store.BenchWall(family, "rate_limit", "You've hit your usage limit"); err != nil {
			t.Fatal(err)
		}
	}
	var logs []string
	plan := llmroute.Plan{Candidates: []string{"codex-tmux", "claude-tmux"}}
	out := bridgechain.ApplyCLIHealthBench(root, "audit", plan, now, func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) })
	if len(out.Candidates) != 2 || len(logs) != 1 || !strings.Contains(logs[0], "ALL candidates benched") {
		t.Fatalf("out=%v logs=%q, want both candidates kept and the all-benched WARN", out.Candidates, logs)
	}
}

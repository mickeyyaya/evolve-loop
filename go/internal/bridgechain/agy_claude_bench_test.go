package bridgechain_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func TestBenchOnEscalation_AQuotaWallOnOneAgyTargetLeavesTheOtherFirst(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC) }
	plan := llmroute.Plan{Candidates: []string{"agy-tmux", "agy-claude-tmux", "claude-tmux"}}
	cases := []struct {
		walled, benched string
		want            []string
	}{
		{"agy-tmux", "agy", []string{"agy-claude-tmux", "claude-tmux", "agy-tmux"}},
		{"agy-claude-tmux", "agy-claude", []string{"agy-tmux", "claude-tmux", "agy-claude-tmux"}},
	}
	for _, tc := range cases {
		root, ws := t.TempDir(), t.TempDir()
		report, _ := json.Marshal(map[string]any{"captured_at": now(), "cli": tc.walled, "pattern_name": "rate_limit", "pane_tail": "RESOURCE_EXHAUSTED"})
		if err := os.WriteFile(filepath.Join(ws, "escalation-report.json"), report, 0o644); err != nil {
			t.Fatal(err)
		}

		bridgechain.BenchOnEscalation(bridgechain.Escalation{ProjectRoot: root, Workspace: ws, CLI: tc.walled, DispatchStart: now().Add(-time.Minute), Env: map[string]string{}}, now, nil)

		active := clihealth.NewStore(root, now).Active()
		if _, benched := active[tc.benched]; !benched || len(active) != 1 {
			t.Errorf("wall on %s: benches = %v, want %s alone: Antigravity pools quota per model group (Gemini; Claude and GPT)", tc.walled, active, tc.benched)
		}
		out := bridgechain.ApplyCLIHealthBench(root, "build", plan, map[string]string{}, now, nil)
		if !slices.Equal(out.Candidates, tc.want) {
			t.Errorf("wall on %s: chain = %v, want %v", tc.walled, out.Candidates, tc.want)
		}
	}
}

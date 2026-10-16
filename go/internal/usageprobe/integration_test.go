package usageprobe_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

func TestProbeToDispatchDemotion(t *testing.T) {
	store := clihealth.NewStore(t.TempDir(), nil)

	p := &usageprobe.Prober{
		Families: []string{"codex"},
		Probe: func(_ context.Context, _ string) (string, error) {
			return "5h limit: 0% left (resets 14:39)", nil
		},
		Classify: func(_, pane string) bool { return pane != "" },
		Store:    store,
		Log:      io.Discard,
	}
	p.Run(context.Background())

	benched := map[string]time.Time{}
	for fam, e := range store.Active() {
		benched[fam] = e.BenchedAt
	}
	plan := llmroute.Plan{Candidates: []string{"codex-tmux", "claude-tmux"}}
	out := llmroute.ApplyBench(plan, benched)

	if len(out.Candidates) == 0 || out.Candidates[0] != "claude-tmux" {
		t.Fatalf("chain=%v, want claude-tmux first (codex demoted by the probe's bench)", out.Candidates)
	}
}

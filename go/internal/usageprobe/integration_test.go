package usageprobe_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/clihealth"
	"github.com/mickeyyaya/evolveloop/go/internal/llmroute"
	"github.com/mickeyyaya/evolveloop/go/internal/usageprobe"
)

// TestProbeToDispatchDemotion is the end-to-end proof that the probe removes the
// wasted boot: a capped family detected by the probe is benched into the
// clihealth store, and the dispatcher's own pre-skip (llmroute.ApplyBench, which
// reads that store) then demotes it so a healthy CLI leads the chain. No tmux —
// the seam between "probe benched it" and "dispatch skips it" is what matters.
func TestProbeToDispatchDemotion(t *testing.T) {
	store := clihealth.NewStore(t.TempDir(), nil)

	p := &usageprobe.Prober{
		Families: []string{"codex"},
		Probe: func(_ context.Context, _ string) (string, error) {
			return "5h limit: 0% left (resets 14:39)", nil // capped
		},
		Classify: func(_, pane string) bool { return pane != "" },
		Store:    store,
		Log:      io.Discard,
	}
	p.Run(context.Background())

	// The dispatcher reads the same store to demote benched families.
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

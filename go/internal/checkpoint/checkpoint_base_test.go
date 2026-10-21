package checkpoint

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestBoundaryCheckpointers_RecordTheCycleBaseAsGitHead(t *testing.T) {
	writers := map[string]func(core.CycleState, string, time.Time) error{
		"quota-likely":       core.QuotaBoundaryCheckpointer,
		"operator-requested": core.ResumeBoundaryCheckpointer,
		"phase-complete":     core.PhaseBoundaryCheckpointer,
	}
	for name, write := range writers {
		for base, cs := range map[string]func(core.CycleState) core.CycleState{
			"7aa5456e2": func(cs core.CycleState) core.CycleState {
				cs.WorktreeBaseSHA, cs.PreCycleHEAD = "7aa5456e2", "1111111"
				return cs
			},
			"1111111": func(cs core.CycleState) core.CycleState {
				cs.WorktreeBaseSHA, cs.PreCycleHEAD = "", "1111111"
				return cs
			},
		} {
			t.Run(name+" "+base, func(t *testing.T) {
				root, fixture := quotaCheckpointFixture(t)

				if err := write(cs(fixture), root, time.Date(2026, 10, 20, 14, 53, 43, 0, time.UTC)); err != nil {
					t.Fatal(err)
				}

				if got := readBlock(t, filepath.Join(root, ".evolve", "cycle-state.json")).GitHead; got != base {
					t.Fatalf("gitHead = %q, want %q: the resume check compares this base with origin/main, and an empty gitHead disabled it", got, base)
				}
			})
		}
	}
}

package cyclehealth

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func TestClassifyOutcome_AFleetLaneDeferredForItsWorktreeIsDeferred(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, timing string
		want         Outcome
	}{
		{"the host ending of a lane that never provisioned its worktree", `[{"phase":"start","verdict":"SKIPPED","abort_reason":"` + cyclestate.CycleTerminationLaneWorktreeDeferred + `: worktree base ref (cycle 1806): git fetch origin main: rc=1"}]`, OutcomeDeferred},
		{"a worktree failure that dispatched phases stays explained", `[{"phase":"scout","verdict":"FAIL","abort_reason":"phase scout: fleet mode: explicit worktree required"}]`, OutcomeFailedExplained},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeTiming(t, dir, tc.timing)
			if got, detail := ClassifyOutcome(dir); got != tc.want {
				t.Errorf("outcome=%s (detail=%q), want %s", got, detail, tc.want)
			}
		})
	}
}

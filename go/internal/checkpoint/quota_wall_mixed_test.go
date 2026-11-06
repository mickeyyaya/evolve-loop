package checkpoint

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestWithQuotaReset_AMixedWallWithAnUnknownResetNamesTheLoginStep(t *testing.T) {
	withUsageReset(t, nil)
	root, cs := quotaCheckpointFixture(t)
	wall := core.Wall{}.Observe(core.FamilyEnd{CLI: "claude-tmux", Pattern: "auth_recheck"}).Observe(core.FamilyEnd{CLI: "agy", Pattern: "rate_limit"})
	cs.QuotaWall = wall.Ends
	if err := core.QuotaBoundaryCheckpointer(cs, root, time.Date(2026, 10, 9, 18, 42, 13, 0, time.Local)); err != nil {
		t.Fatal(err)
	}
	if got, want := readPause(t, root).OperatorAction, core.WallNextStep(wall, cs.WorkspacePath); got != want {
		t.Errorf("action = %q, want %q", got, want)
	}
}

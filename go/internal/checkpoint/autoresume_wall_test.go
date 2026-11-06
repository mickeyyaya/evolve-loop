package checkpoint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestSpentAction_RendersTheWallAndNotACauseOfItsOwn(t *testing.T) {
	cases := []struct {
		name string
		cp   Checkpoint
		want string
	}{
		{"an older checkpoint with no wall", Checkpoint{}, "the auto-resume budget is spent: no cause was recorded for this pause. Next: evolve loop --resume."},
		{"a wall of one login", Checkpoint{QuotaWall: []core.FamilyEnd{{CLI: "claude", Pattern: "auth_recheck"}}}, "the auto-resume budget is spent: claude needs a login. Next: open claude and run /login. Then evolve loop --resume."},
		{"a wall of one quota", Checkpoint{QuotaWall: []core.FamilyEnd{{CLI: "agy", Pattern: "rate_limit"}}}, "the auto-resume budget is spent: agy is on quota, reset unknown. Next: wait for agy, reset unknown. Then evolve loop --resume."},
	}
	for _, c := range cases {
		if got := spentAction(c.cp); got != c.want {
			t.Errorf("%s: spentAction = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestWithAutoResumeBudget_ASpentBudgetKeepsTheLoginStepOfTheWall(t *testing.T) {
	cp := Checkpoint{AutoResumeMaxAttempts: 3, QuotaWall: []core.FamilyEnd{{CLI: "claude", Pattern: "auth_recheck"}}, OperatorAction: "kept"}
	if got := withAutoResumeBudget(cp, 3).OperatorAction; got != "kept" {
		t.Errorf("OperatorAction = %q, want the action already set kept", got)
	}
	cp.OperatorAction = ""
	if got := withAutoResumeBudget(cp, 3).OperatorAction; got != spentAction(cp) {
		t.Errorf("OperatorAction = %q, want %q", got, spentAction(cp))
	}
}

func TestSpentAction_ALaneCheckpointNamesTheLaneResumeOfItsWorkspace(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, core.LaneScopeFile), []byte(`{"todo_ids":["a"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cs := core.CycleState{WorkspacePath: ws}
	cp := Compose(cs, ReasonQuotaLikely, 0, "head", time.Now())
	cp.QuotaWall = []core.FamilyEnd{{CLI: "claude", Pattern: "auth_recheck"}}

	got := spentAction(cp)

	want := "the auto-resume budget is spent: claude needs a login. Next: open claude and run /login. Then evolve cycle resume --all, then evolve wave next."
	if cp.WorkspacePath != ws || got != want {
		t.Errorf("workspace=%q spentAction = %q, want %q", cp.WorkspacePath, got, want)
	}
}

func TestWithQuotaReset_AWallOfOnlyAnotherCauseNamesNoQuotaInTheUnknownResetAction(t *testing.T) {
	withUsageReset(t, nil)
	root, cs := quotaCheckpointFixture(t)
	wall := core.Wall{}.Observe(core.FamilyEnd{CLI: "claude-tmux", Pattern: "bridge_crash"})
	cs.QuotaWall = wall.Ends
	if err := core.QuotaBoundaryCheckpointer(cs, root, time.Date(2026, 10, 9, 18, 42, 13, 0, time.Local)); err != nil {
		t.Fatal(err)
	}

	got := readPause(t, root).OperatorAction

	if want := core.WallNextStep(wall, cs.WorkspacePath); got != want {
		t.Errorf("action = %q, want %q: a wall with no quota row must not name quota", got, want)
	}
	if strings.Contains(got, "quota") {
		t.Errorf("action %q names quota for a wall that holds no quota row", got)
	}
}

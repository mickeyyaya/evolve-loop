package guards

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestProtectedSurfaceManifest_CoversTheSeatDirectory(t *testing.T) {
	for _, path := range []string{
		"/home/u/.evolve-seat",
		"/home/u/.evolve-seat/",
		"/home/u/.evolve-seat/holder",
		"/home/u/.evolve-seat/holder/token",
		"/wt/cycle-9/.evolve-seat/holder/token",
	} {
		if !IsProtectedSurface(path) {
			t.Errorf("IsProtectedSurface(%q) = false: the seat directory holds the token of the claude seats, so no cycle may write it", path)
		}
	}
	if IsProtectedSurface("/home/u/.evolve-seatbelt/x") {
		t.Error("the fragment must name the directory, not any name that starts with it")
	}
}

func TestProtectedSurfaceManifest_CoversTheCodeThatHoldsTheSeatDeny(t *testing.T) {
	for _, path := range []string{
		"/repo/go/internal/bridge/seat_holder.go",
		"/repo/go/internal/paths/seat.go",
		"/repo/go/internal/bridge/seat_holder_test.go",
		"/repo/go/internal/paths/seat_test.go",
	} {
		if !IsProtectedSurface(path) {
			t.Errorf("IsProtectedSurface(%q) = false: a build that edits it could turn the holder deny into a no-op", path)
		}
	}
}

func TestRole_ABuildCannotWriteUnderTheSeatDirectoryEvenInsideItsWorktree(t *testing.T) {
	worktree := "/work/wt/cycle-42"
	s := cycleStateOnly{cs: core.CycleState{
		CycleID: 42, Phase: "build", ActiveAgent: "builder",
		ActiveWorktree: worktree, WorkspacePath: "/work/evolve/runs/cycle-42",
	}}
	g := newRoleWithHome(s, false, fixtureHome)
	for _, tool := range []string{"Edit", "Write"} {
		dec := g.Decide(context.Background(), core.GuardInput{
			ToolName:  tool,
			ToolInput: map[string]any{"file_path": filepath.Join(worktree, ".evolve-seat", "holder", "token")},
		})
		if dec.Allow || !dec.Alarm {
			t.Errorf("%s under /.evolve-seat/ by a build: allow=%v alarm=%v, want a refusal with an alarm", tool, dec.Allow, dec.Alarm)
		}
	}
}

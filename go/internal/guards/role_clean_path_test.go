package guards

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// cycleStateOnly serves one cycle state and nothing else, so a fixture workspace needs no directory
// and can sit outside every always-safe dir on every OS.
type cycleStateOnly struct {
	core.Storage
	cs core.CycleState
}

func (s cycleStateOnly) ReadCycleState(context.Context) (core.CycleState, error) { return s.cs, nil }

func TestRole_DeniesADotDotPathIntoTheControlPlane(t *testing.T) {
	worktree := "/work/wt/cycle-60"
	s := cycleStateOnly{cs: core.CycleState{
		CycleID:        60,
		Phase:          "build",
		ActiveWorktree: worktree,
		WorkspacePath:  "/work/evolve/runs/cycle-60",
	}}
	g := newRoleWithHome(s, false, fixtureHome)

	for _, path := range []string{
		worktree + "/go/internal/core/../guards/role.go",
		worktree + "/go/internal/./guards/ship.go",
		worktree + "/go//internal/guards/quota.go",
	} {
		dec := g.Decide(context.Background(), core.GuardInput{
			ToolName:  "Write",
			ToolInput: map[string]any{"file_path": path},
		})
		if dec.Allow || !dec.Alarm {
			t.Errorf("Write %q: allow=%v alarm=%v, want an alarmed deny (it names %s)", path, dec.Allow, dec.Alarm, filepath.Clean(path))
		}
	}
}

func TestRole_ADotDotOutOfAnAlwaysSafeDirIsNotAlwaysSafe(t *testing.T) {
	s := cycleStateOnly{cs: core.CycleState{
		CycleID:       61,
		Phase:         "build",
		WorkspacePath: "/work/evolve/runs/cycle-61",
	}}
	g := newRoleWithHome(s, false, fixtureHome)

	for _, path := range []string{
		"/tmp/../work/repo/README.md",
		fixtureHome + "/.claude/../.ssh/authorized_keys",
	} {
		dec := g.Decide(context.Background(), core.GuardInput{
			ToolName:  "Edit",
			ToolInput: map[string]any{"file_path": path},
		})
		if dec.Allow {
			t.Errorf("Edit %q allowed: it names %s, outside every always-safe dir and the workspace", path, filepath.Clean(path))
		}
	}
}

func TestIsProtectedSurface_JudgesTheCleanPath(t *testing.T) {
	cases := map[string]bool{
		"/wt/go/internal/core/../guards/role.go": true,
		"go/internal/core/../guards/role.go":     true,
		"../go/internal/guards/role.go":          true,
		"/wt/go/internal/guards/../core/foo.go":  false,
	}
	for path, want := range cases {
		if got := IsProtectedSurface(path); got != want {
			t.Errorf("IsProtectedSurface(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestIsProtectedScope_KeepsTheDirectorySpellingOfACleanPath(t *testing.T) {
	if !IsProtectedScope("go/internal/core/../../internal/") {
		t.Error("a directory spelling that cleans to go/internal/ contains protected surface and must stay in scope")
	}
}

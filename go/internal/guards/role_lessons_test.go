package guards

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func lessonsGuardInput(path string) core.GuardInput {
	return core.GuardInput{ToolName: "Write", ToolInput: map[string]any{"file_path": path}}
}

func lessonsRole(t *testing.T, phase string) (*Role, string) {
	t.Helper()
	root := t.TempDir()
	s, _ := setupStorageWithCS(t, core.CycleState{
		CycleID: 7, Phase: phase,
		WorkspacePath: filepath.Join(root, ".evolve", "runs", "cycle-7"),
	})
	return NewRole(s, false), root
}

func TestRole_RetroMayWriteLessonsCorpus(t *testing.T) {
	r, _ := lessonsRole(t, "retro")
	// Not a t.TempDir() path: on Linux that sits under /tmp and would ride the always-safe allowance.
	d := r.Decide(context.Background(), lessonsGuardInput("/repo/.evolve/instincts/lessons/cycle-7-lesson.yaml"))
	if !d.Allow {
		t.Fatalf("retro must be allowed to write the lessons corpus, got deny: %s", d.Reason)
	}
}

func TestRole_NonRetroDeniedLessonsCorpus(t *testing.T) {
	r, _ := lessonsRole(t, "build")
	if d := r.Decide(context.Background(), lessonsGuardInput("/repo/.evolve/instincts/lessons/cycle-7-lesson.yaml")); d.Allow {
		t.Fatal("non-retro phases must not ride the lessons allowance")
	}
}

func TestRole_LessonsAllowanceNeverCoversProtectedSurface(t *testing.T) {
	r, _ := lessonsRole(t, "retro")
	d := r.Decide(context.Background(), lessonsGuardInput("/p/go/internal/guards/role.go"))
	if d.Allow {
		t.Fatal("protected surface must stay denied in retro phase")
	}
}

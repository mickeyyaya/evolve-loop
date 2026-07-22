package guards

// role_lessons_test.go — unit pins for the retro lessons-corpus allowance
// (cycles 1036/1041/1042: the header documented the allowance, Decide never
// implemented it, so retros could not persist lesson YAMLs and learning
// durability silently broke). The ACS suites acs/cycle{1036,1041,1042} carry
// the behavioral contract under -tags acs; these pins keep the crux in the
// default sweep.

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
		// setupStorageWithCS mirrors run.json into the workspace — must be a
		// real writable dir.
		WorkspacePath: filepath.Join(root, ".evolve", "runs", "cycle-7"),
	})
	return NewRole(s, false), root
}

func TestRole_RetroMayWriteLessonsCorpus(t *testing.T) {
	r, _ := lessonsRole(t, "retro")
	// Fixed non-/tmp path: Decide never stats the write path, and a t.TempDir()
	// path would ride the /tmp always-safe allowance on Linux (the exact
	// platform split that flunked PR #351's ubuntu job while macos passed).
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
	// Ordering pin: the control-plane boundary is checked BEFORE the lessons
	// allowance — a protected path can never ride it, retro phase or not.
	r, _ := lessonsRole(t, "retro")
	d := r.Decide(context.Background(), lessonsGuardInput("/p/go/internal/guards/role.go"))
	if d.Allow {
		t.Fatal("protected surface must stay denied in retro phase")
	}
}

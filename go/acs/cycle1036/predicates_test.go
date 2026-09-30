//go:build acs

package cycle1036

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const modPrefix = "github.com/mickeyyaya/evolve-loop/go/"

const synthRoot = "/synth/repo-cycle1036"

func workspacePath() string {
	return filepath.Join(synthRoot, ".evolve", "runs", "cycle-1036")
}

func lessonsDir() string {
	return filepath.Join(synthRoot, ".evolve", "instincts", "lessons")
}

type fakeStorage struct{ cs core.CycleState }

func (f fakeStorage) ReadState(context.Context) (core.State, error) { return core.State{}, nil }
func (f fakeStorage) WriteState(context.Context, core.State) error  { return nil }
func (f fakeStorage) ReadCycleState(context.Context) (core.CycleState, error) {
	return f.cs, nil
}
func (f fakeStorage) WriteCycleState(context.Context, core.CycleState) error { return nil }
func (f fakeStorage) AcquireLock(context.Context) (func() error, error) {
	return func() error { return nil }, nil
}

func retroRole() *guards.Role {
	cs := core.CycleState{
		CycleID:       1036,
		Phase:         string(core.PhaseRetro),
		WorkspacePath: workspacePath(),
	}
	return guards.NewRole(fakeStorage{cs: cs}, false)
}

func decideWrite(path string) core.GuardDecision {
	return retroRole().Decide(context.Background(), core.GuardInput{
		ToolName:  "Write",
		ToolInput: map[string]any{"file_path": path},
	})
}

func TestC1036_001_retro_may_write_under_lessons_dir(t *testing.T) {
	path := filepath.Join(lessonsDir(), "retro-role-gate-lessons-write-allowance.yaml")
	dec := decideWrite(path)
	if !dec.Allow {
		t.Errorf("RED: retro-phase write under the lesson corpus %s was denied (Reason=%q); "+
			"the role guard's promised learn/retrospective lessons-write allowance is unimplemented",
			path, dec.Reason)
	}
	if dec.Alarm {
		t.Errorf("RED: a legitimate lessons write raised an integrity Alarm (Reason=%q) — "+
			"the lessons path was misclassified as a protected surface", dec.Reason)
	}
}

func TestC1036_002_retro_write_outside_lessons_and_workspace_denied(t *testing.T) {
	path := filepath.Join(synthRoot, "go", "internal", "core", "cyclerun.go")
	dec := decideWrite(path)
	if dec.Allow {
		t.Errorf("retro-phase write outside workspace+lessons (%s) was allowed — "+
			"the lessons allowance over-broadened into general repo-tree writes", path)
	}
}

func TestC1036_003_lessons_path_cannot_smuggle_protected_surface(t *testing.T) {
	path := filepath.Join(lessonsDir(), ".evolve", "policy.json")
	if !guards.IsProtectedSurface(path) {
		t.Fatalf("test premise broken: %s is expected to be a protected surface", path)
	}
	dec := decideWrite(path)
	if dec.Allow {
		t.Errorf("SMUGGLE: a protected control-plane path under the lessons dir (%s) was ALLOWED — "+
			"the lessons allowance bypassed the IsProtectedSurface deny", path)
	}
	if !dec.Alarm {
		t.Errorf("a protected-surface deny under the lessons dir (%s) did not raise an integrity Alarm", path)
	}
}

func TestC1036_004_lessons_path_traversal_escape_denied(t *testing.T) {
	path := filepath.Join(lessonsDir(), "..", "..", "..", "etc", "passwd")
	dec := decideWrite(path)
	if dec.Allow {
		t.Errorf("TRAVERSAL: a path escaping the lessons dir via `..` (%s) was allowed — "+
			"the allowance used a naive prefix match instead of clean containment", path)
	}
}

// acs-predicate: config-check — the deliverable of AC2 IS the documentation
func TestC1036_005_role_doc_comment_names_real_lessons_path(t *testing.T) {
	roleGo := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "guards", "role.go")
	if !acsassert.FileExists(t, roleGo) {
		return
	}
	raw, rerr := os.ReadFile(roleGo)
	if rerr != nil {
		t.Fatalf("read role.go: %v", rerr)
	}
	if !strings.Contains(string(raw), ".evolve/instincts/lessons") {
		t.Errorf("role.go doc comment does not name the real lesson corpus path `.evolve/instincts/lessons` (kb.go:75)")
	}
	if strings.Contains(string(raw), ".evolve/lessons/**") {
		t.Errorf("role.go doc comment still carries the stale `.evolve/lessons/**` path")
	}
}

func TestC1036_006_role_guard_suite_no_regression(t *testing.T) {
	pkg := modPrefix + "internal/guards"
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", pkg, "-run", "TestRole", "-count=1", "-v")
	if code != 0 || err != nil {
		t.Errorf("`go test %s -run TestRole` exited %d (err=%v) — role guard regression\nstdout:\n%s\nstderr:\n%s",
			pkg, code, err, stdout, stderr)
		return
	}
	if !strings.Contains(stdout, "--- PASS: TestRole") {
		t.Errorf("`go test %s -run TestRole` reported no PASS marker — the filter matched zero tests", pkg)
	}
}

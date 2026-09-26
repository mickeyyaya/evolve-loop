package guards

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// fixtureHome is a synthetic operator home: it cannot exist, cannot be under /tmp and cannot vary by
// machine, so a <home>/.claude test exercises that rule and never the /tmp one.
const fixtureHome = "/fixture-home/operator"

// The t.Setenv is a tripwire: these assertions fail only if isAlwaysSafe starts reading the environment.
func TestIsAlwaysSafe_DecidesAgainstTheGivenHome(t *testing.T) {
	if !isAlwaysSafe(filepath.Join(fixtureHome, ".claude", "somefile"), fixtureHome) {
		t.Errorf("%s/.claude/somefile is not always-safe against its OWN home", fixtureHome)
	}
	t.Setenv("HOME", "/some/other/home")
	if !isAlwaysSafe(filepath.Join(fixtureHome, ".claude", "somefile"), fixtureHome) {
		t.Error("the given home stopped working once the ambient HOME disagreed — something is reading the environment again")
	}
	if isAlwaysSafe(filepath.Join("/some/other/home", ".claude", "somefile"), fixtureHome) {
		t.Error("a DIFFERENT home's .claude was treated as always-safe — the rule must be scoped to one home")
	}
}

func TestIsAlwaysSafe_UnsetHomeNeverMatchesClaude(t *testing.T) {
	for _, path := range []string{
		".claude/settings.json",
		"/repo/.claude/settings.json",
		"/Users/someone/.claude/settings.json",
	} {
		if isAlwaysSafe(path, "") {
			t.Errorf("isAlwaysSafe(%q, home=\"\") = true — with no home there is no $HOME/.claude rule to apply", path)
		}
	}
	if !isAlwaysSafe("/tmp/scratch/foo.go", "") {
		t.Error("/tmp scratch must stay always-safe regardless of home")
	}
}

func TestNewRole_ResolvesHomeFromEnv(t *testing.T) {
	t.Setenv("HOME", fixtureHome)
	g := NewRole(nil, false)
	if g.home != fixtureHome {
		t.Errorf("NewRole resolved home=%q, want %q from $HOME", g.home, fixtureHome)
	}
	t.Setenv("HOME", "")
	if g := NewRole(nil, false); g.home != "" {
		t.Errorf("with HOME unset NewRole resolved home=%q, want \"\" (no fabricated fallback — /tmp was the old one, and it made $HOME tests assert the /tmp rule)", g.home)
	}
}

func TestRole_Name(t *testing.T) {
	g := NewRole(nil, false)
	if g.Name() != "role" {
		t.Errorf("name=%q", g.Name())
	}
}

func TestRole_BuilderWritesInWorktree(t *testing.T) {
	worktree := "/tmp/wt/cycle-42"
	s, _ := setupStorageWithCS(t, core.CycleState{
		CycleID:        42,
		Phase:          "build",
		ActiveAgent:    "builder",
		ActiveWorktree: worktree,
		WorkspacePath:  filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-42"),
	})
	g := NewRole(s, false)

	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": filepath.Join(worktree, "src/foo.go")},
	})
	if !dec.Allow {
		t.Errorf("builder write in worktree denied: %s", dec.Reason)
	}

	dec = g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": "/Users/x/some/other/file.go"},
	})
	if dec.Allow {
		t.Error("builder write outside worktree+workspace must deny")
	}
}

func TestRole_TDDWritesTestsInWorktree(t *testing.T) {
	worktree := "/work/wt/cycle-50" // non-/tmp so isAlwaysSafe doesn't short-circuit
	s, _ := setupStorageWithCS(t, core.CycleState{
		CycleID:        50,
		Phase:          "tdd",
		ActiveAgent:    "tdd-engineer",
		ActiveWorktree: worktree,
		WorkspacePath:  filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-50"),
	})
	g := NewRole(s, false)

	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Write",
		ToolInput: map[string]any{"file_path": filepath.Join(worktree, "go/internal/textutil/new_behavior_test.go")},
	})
	if !dec.Allow {
		t.Errorf("tdd write of *_test.go in worktree denied: %s", dec.Reason)
	}
}

func TestRole_NonWorktreePhaseDeniedWorktreeWrite(t *testing.T) {
	worktree := "/work/wt/cycle-51" // non-/tmp so isAlwaysSafe doesn't short-circuit
	s, _ := setupStorageWithCS(t, core.CycleState{
		CycleID:        51,
		Phase:          "scout",
		ActiveWorktree: worktree,
		WorkspacePath:  filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-51"),
	})
	g := NewRole(s, false)

	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Write",
		ToolInput: map[string]any{"file_path": filepath.Join(worktree, "go/internal/foo/bar.go")},
	})
	if dec.Allow {
		t.Error("scout (non-worktree phase) must not write source into the worktree")
	}
}

func TestRole_AuditPhaseRestricted(t *testing.T) {
	ws := filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-7")
	s, _ := setupStorageWithCS(t, core.CycleState{
		CycleID:       7,
		Phase:         "audit",
		WorkspacePath: ws,
	})
	g := NewRole(s, false)

	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": filepath.Join(ws, "audit-report.md")},
	})
	if !dec.Allow {
		t.Errorf("audit-report write denied: %s", dec.Reason)
	}

	dec = g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": "/repo/src/foo.go"},
	})
	if dec.Allow {
		t.Error("audit phase writing source file must deny")
	}
}

func TestRole_AlwaysSafeDirs(t *testing.T) {
	s, _ := setupStorageWithCS(t, core.CycleState{CycleID: 1, Phase: "build"})
	g := newRoleWithHome(s, false, fixtureHome)
	for _, path := range []string{
		"/tmp/scratch/foo.go",
		filepath.Join(fixtureHome, ".claude/somefile"),
	} {
		dec := g.Decide(context.Background(), core.GuardInput{
			ToolName:  "Write",
			ToolInput: map[string]any{"file_path": path},
		})
		if !dec.Allow {
			t.Errorf("always-safe %q denied: %s", path, dec.Reason)
		}
		if dec.Alarm {
			t.Errorf("always-safe %q raised an Alarm: %s — alarming on ordinary scratch writes floods the operator signal that real violations need", path, dec.Reason)
		}
	}
}

func TestRole_NonEditWriteToolsPass(t *testing.T) {
	s, _ := setupStorageWithCS(t, core.CycleState{CycleID: 1, Phase: "build"})
	g := NewRole(s, false)
	for _, tool := range []string{"Bash", "Read", "Grep", "Glob"} {
		dec := g.Decide(context.Background(), core.GuardInput{ToolName: tool})
		if !dec.Allow {
			t.Errorf("tool=%s denied: %s", tool, dec.Reason)
		}
	}
}

func TestRole_BypassAllows(t *testing.T) {
	s, _ := setupStorageWithCS(t, core.CycleState{CycleID: 1, Phase: "build",
		WorkspacePath: filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-1")})
	g := NewRole(s, true)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": "/some/forbidden/path.go"},
	})
	if !dec.Allow {
		t.Errorf("bypass must allow, got: %s", dec.Reason)
	}
}

func TestRole_OutsideCyclePasses(t *testing.T) {
	s, _ := setupStorageNoCS(t)
	g := NewRole(s, false)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": "/repo/src/foo.go"},
	})
	if !dec.Allow {
		t.Errorf("outside cycle must allow: %s", dec.Reason)
	}
}

func TestRole_NilStorageDenies(t *testing.T) {
	g := NewRole(nil, false)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": "/foo"},
	})
	if dec.Allow {
		t.Error("nil storage must deny by default")
	}
}

func TestRole_ReadCycleStateErrorDenies(t *testing.T) {
	s, _ := setupStorageNoCS(t)
	g := NewRole(erroringStorage{s}, false)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": "/foo"},
	})
	if dec.Allow {
		t.Error("read error must deny")
	}
}

func TestIsUnderDir_EmptyDir(t *testing.T) {
	if isUnderDir("/foo", "") {
		t.Error("empty dir must report false")
	}
}

func TestIsUnderDir_SamePath(t *testing.T) {
	if !isUnderDir("/foo", "/foo") {
		t.Error("same path must report true")
	}
}

func TestIsUnderDir_OutsidePath(t *testing.T) {
	if isUnderDir("/other/file", "/dir") {
		t.Error("/other/file under /dir must report false")
	}
}

func TestRole_BuildPhaseNoWorktree_DeniesNonWorkspace(t *testing.T) {
	s, _ := setupStorageWithCS(t, core.CycleState{
		CycleID:        1,
		Phase:          "build",
		ActiveWorktree: "",
		WorkspacePath:  filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-1"),
	})
	g := NewRole(s, false)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": "/some/random.go"},
	})
	if dec.Allow {
		t.Error("build phase without worktree must deny non-workspace writes")
	}
}

func TestRole_MissingFilePathAllows(t *testing.T) {
	s, _ := setupStorageWithCS(t, core.CycleState{CycleID: 1, Phase: "build"})
	g := NewRole(s, false)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{},
	})
	if !dec.Allow {
		t.Errorf("missing file_path must allow, got: %s", dec.Reason)
	}
}

func TestRoleGuard_RelativeWorkspacePath(t *testing.T) {
	// Run from a temp cwd: WriteCycleState mirrors run.json into the relative workspace, which would
	// otherwise land in the package source tree.
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	abs, err := filepath.Abs("./.evolve/runs/cycle-107")
	if err != nil {
		t.Fatal(err)
	}
	s, _ := setupStorageWithCS(t, core.CycleState{
		CycleID:       107,
		Phase:         "scout",
		WorkspacePath: "./.evolve/runs/cycle-107",
	})
	g := NewRole(s, false)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Write",
		ToolInput: map[string]any{"file_path": filepath.Join(abs, "scout-report.md")},
	})
	if !dec.Allow {
		t.Errorf("relative workspace_path: abs write must allow: %s", dec.Reason)
	}
	dec = g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Write",
		ToolInput: map[string]any{"file_path": "/etc/hosts"},
	})
	if dec.Allow {
		t.Error("write outside workspace must deny")
	}
}

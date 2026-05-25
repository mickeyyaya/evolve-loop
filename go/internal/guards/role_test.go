package guards

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return "/tmp"
}

// Role is the port of scripts/guards/role-gate.sh — per-phase write
// allowlists for the Edit/Write tools. Phase-1 subset of rules:
//   - calibrate/research/discover: workspace only
//   - build: workspace + active_worktree/**
//   - audit: workspace + audit-* artifacts at evolve dir
//   - learn/retrospective: orchestrator-report.md, lessons/*.yaml,
//     state.json
//   - Always-safe: /tmp/**, $HOME/.claude/**
//   - Read/Bash tools pass through (different guards handle them)
//   - EVOLVE_BYPASS_ROLE_GATE=1 bypasses
func TestRole_Name(t *testing.T) {
	g := NewRole(nil)
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
		WorkspacePath:  "/tmp/wt/.evolve/runs/cycle-42",
	})
	g := NewRole(s)

	// In worktree: allow.
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": filepath.Join(worktree, "src/foo.go")},
	})
	if !dec.Allow {
		t.Errorf("builder write in worktree denied: %s", dec.Reason)
	}

	// Outside worktree, outside workspace: deny.
	dec = g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": "/Users/x/some/other/file.go"},
	})
	if dec.Allow {
		t.Error("builder write outside worktree+workspace must deny")
	}
}

func TestRole_AuditPhaseRestricted(t *testing.T) {
	s, _ := setupStorageWithCS(t, core.CycleState{
		CycleID:       7,
		Phase:         "audit",
		WorkspacePath: "/tmp/wt/.evolve/runs/cycle-7",
	})
	g := NewRole(s)

	// audit-report.md inside workspace: allow.
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": "/tmp/wt/.evolve/runs/cycle-7/audit-report.md"},
	})
	if !dec.Allow {
		t.Errorf("audit-report write denied: %s", dec.Reason)
	}

	// random source file outside workspace: deny.
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
	g := NewRole(s)
	for _, path := range []string{
		"/tmp/scratch/foo.go",
		filepath.Join(homeDir(), ".claude/somefile"),
	} {
		dec := g.Decide(context.Background(), core.GuardInput{
			ToolName:  "Write",
			ToolInput: map[string]any{"file_path": path},
		})
		if !dec.Allow {
			t.Errorf("always-safe %q denied: %s", path, dec.Reason)
		}
	}
}

func TestRole_NonEditWriteToolsPass(t *testing.T) {
	s, _ := setupStorageWithCS(t, core.CycleState{CycleID: 1, Phase: "build"})
	g := NewRole(s)
	for _, tool := range []string{"Bash", "Read", "Grep", "Glob"} {
		dec := g.Decide(context.Background(), core.GuardInput{ToolName: tool})
		if !dec.Allow {
			t.Errorf("tool=%s denied: %s", tool, dec.Reason)
		}
	}
}

func TestRole_BypassEnvAllows(t *testing.T) {
	t.Setenv("EVOLVE_BYPASS_ROLE_GATE", "1")
	s, _ := setupStorageWithCS(t, core.CycleState{CycleID: 1, Phase: "build", WorkspacePath: "/tmp/wt/.evolve/runs/cycle-1"})
	g := NewRole(s)
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
	g := NewRole(s)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": "/repo/src/foo.go"},
	})
	if !dec.Allow {
		t.Errorf("outside cycle must allow: %s", dec.Reason)
	}
}

func TestRole_NilStorageDenies(t *testing.T) {
	g := NewRole(nil)
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
	g := NewRole(erroringStorage{s})
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
		CycleID:       1,
		Phase:         "build",
		ActiveWorktree: "",
		WorkspacePath: "/tmp/wt/.evolve/runs/cycle-1",
	})
	g := NewRole(s)
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
	g := NewRole(s)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{}, // no file_path
	})
	if !dec.Allow {
		t.Errorf("missing file_path must allow, got: %s", dec.Reason)
	}
}

func TestRoleGuard_RelativeWorkspacePath(t *testing.T) {
	abs, err := filepath.Abs("./.evolve/runs/cycle-107")
	if err != nil {
		t.Fatal(err)
	}
	s, _ := setupStorageWithCS(t, core.CycleState{
		CycleID:       107,
		Phase:         "scout",
		WorkspacePath: "./.evolve/runs/cycle-107",
	})
	g := NewRole(s)
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

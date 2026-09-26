package guards

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRole_DeniesControlPlaneEditInBuildPhase(t *testing.T) {
	worktree := "/work/wt/cycle-20" // non-/tmp so isAlwaysSafe doesn't short-circuit
	s, _ := setupStorageWithCS(t, core.CycleState{
		CycleID:        20,
		Phase:          "build",
		ActiveAgent:    "builder",
		ActiveWorktree: worktree,
		WorkspacePath:  filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-20"),
	})
	g := NewRole(s, false)

	for _, rel := range []string{
		"go/acs/regression/flagreaders/readers_test.go",
		"go/internal/flagregistry/registry_table.go",
		"go/internal/guards/role.go",
		"knowledge-base/research/flag-campaign-plan.json",
		"skills/audit/SKILL.md",
		".claude/settings.json",
		"go/internal/core/orchestrator.go",
	} {
		dec := g.Decide(context.Background(), core.GuardInput{
			ToolName:  "Edit",
			ToolInput: map[string]any{"file_path": filepath.Join(worktree, rel)},
		})
		if dec.Allow {
			t.Errorf("build phase editing control-plane %q must be DENIED", rel)
		}
		if !dec.Alarm {
			t.Errorf("control-plane deny for %q must raise an Alarm", rel)
		}
	}
}

func TestRole_AllowsLegitWorktreeWritesUnderProtection(t *testing.T) {
	worktree := "/work/wt/cycle-20"
	s, _ := setupStorageWithCS(t, core.CycleState{
		CycleID:        20,
		Phase:          "build",
		ActiveAgent:    "builder",
		ActiveWorktree: worktree,
		WorkspacePath:  filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-20"),
	})
	g := NewRole(s, false)

	for _, rel := range []string{
		"go/internal/core/observer.go",
		"go/acs/cycle20/predicates_test.go", // the cycle's own predicates, not under regression/
	} {
		dec := g.Decide(context.Background(), core.GuardInput{
			ToolName:  "Write",
			ToolInput: map[string]any{"file_path": filepath.Join(worktree, rel)},
		})
		if !dec.Allow {
			t.Errorf("legit worktree write %q must be allowed: %s", rel, dec.Reason)
		}
	}
}

func TestRole_OutsideCycleAllowsControlPlane(t *testing.T) {
	s, _ := setupStorageNoCS(t)
	g := NewRole(s, false)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": "/repo/go/acs/regression/flagreaders/readers_test.go"},
	})
	if !dec.Allow {
		t.Errorf("outside a cycle, operator must be able to edit the control plane: %s", dec.Reason)
	}
}

func TestRole_DeniesGlobalSettingsInCycle(t *testing.T) {
	s, _ := setupStorageWithCS(t, core.CycleState{
		CycleID: 30, Phase: "build", ActiveAgent: "builder",
		ActiveWorktree: "/work/wt/cycle-30",
		WorkspacePath:  filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-30"),
	})
	g := newRoleWithHome(s, false, fixtureHome)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": filepath.Join(fixtureHome, ".claude/settings.json")},
	})
	if dec.Allow {
		t.Error("a cycle must NOT rewrite the global ~/.claude/settings.json hook wiring")
	}
	if !dec.Alarm {
		t.Error("global settings.json deny must raise an Alarm")
	}
}

func TestRole_C1_DeniesGlobalSettingsUnsetHome(t *testing.T) {
	s, _ := setupStorageWithCS(t, core.CycleState{
		CycleID: 31, Phase: "build", ActiveAgent: "builder",
		ActiveWorktree: "/work/wt/cycle-31",
		WorkspacePath:  filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-31"),
	})
	g := newRoleWithHome(s, false, "") // HOME unset / sandboxed runner
	for _, path := range []string{
		"/Users/operator/.claude/settings.json",
		"/tmp/.claude/settings.json", // a protected file inside an always-safe dir
	} {
		dec := g.Decide(context.Background(), core.GuardInput{
			ToolName:  "Edit",
			ToolInput: map[string]any{"file_path": path},
		})
		if dec.Allow {
			t.Errorf("with no home resolved, %q was ALLOWED — the hook wiring must be denied by the control-plane boundary regardless of $HOME", path)
		}
		if !dec.Alarm {
			t.Errorf("deny of %q must raise an Alarm", path)
		}
	}
}

func TestRole_AllowsGlobalSettingsOutsideCycle(t *testing.T) {
	s, _ := setupStorageNoCS(t)
	g := newRoleWithHome(s, false, fixtureHome)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": filepath.Join(fixtureHome, ".claude/settings.json")},
	})
	if !dec.Allow {
		t.Errorf("operator (no active cycle) must be able to edit global settings: %s", dec.Reason)
	}
}

func TestRole_BypassProtectedPathAlarms(t *testing.T) {
	g := NewRole(nil, true)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": "/repo/go/acs/regression/flagreaders/readers_test.go"},
	})
	if !dec.Allow {
		t.Error("bypass must still allow (emergency override)")
	}
	if !dec.Alarm {
		t.Error("bypass of a protected path must raise an Alarm")
	}
	dec = g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": "/repo/go/internal/core/foo.go"},
	})
	if !dec.Allow || dec.Alarm {
		t.Errorf("non-protected bypass must allow without alarm (allow=%v alarm=%v)", dec.Allow, dec.Alarm)
	}
}

func TestRole_DeniesRelocatedExplanationCallSitesInBuildPhase(t *testing.T) {
	worktree := "/work/wt/cycle-1630" // non-/tmp so isAlwaysSafe doesn't short-circuit
	s, _ := setupStorageWithCS(t, core.CycleState{
		CycleID:        1630,
		Phase:          "build",
		ActiveAgent:    "builder",
		ActiveWorktree: worktree,
		WorkspacePath:  filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-1630"),
	})
	g := NewRole(s, false)

	for _, rel := range []string{
		"go/internal/core/cyclerun_postreview.go",
		"go/internal/core/resume_execution.go",
		"go/internal/core/resume_bootstrap.go",
		"go/internal/phases/audit/classification.go",
		"go/internal/phases/runner/dispatch.go",
	} {
		dec := g.Decide(context.Background(), core.GuardInput{
			ToolName:  "Edit",
			ToolInput: map[string]any{"file_path": filepath.Join(worktree, rel)},
		})
		if dec.Allow {
			t.Errorf("build phase editing relocated explanation call site %q must be DENIED", rel)
		}
		if !dec.Alarm {
			t.Errorf("relocated explanation call site deny for %q must raise an Alarm", rel)
		}
	}

	// Control: neighbors that hold no lifecycle call site stay writable, so the protection is file-narrow.
	for _, rel := range []string{
		"go/internal/core/cyclerun_record.go",
		"go/internal/core/resume_cursor.go",
	} {
		dec := g.Decide(context.Background(), core.GuardInput{
			ToolName:  "Edit",
			ToolInput: map[string]any{"file_path": filepath.Join(worktree, rel)},
		})
		if !dec.Allow {
			t.Errorf("%q holds no explanation call site and must remain writable", rel)
		}
	}
}

package guards

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestRole_DeniesControlPlaneEditInBuildPhase is the cycle-20 regression: a build
// phase — which IS allowed to write source into its worktree — must STILL be
// denied (with an alarm) from editing the gate/metric/guard/contract that grades
// it. The control-plane boundary overrides the worktree allowance.
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
		"go/acs/regression/flagreaders/readers_test.go", // the EXACT cycle-20 breach file
		"go/internal/flagregistry/registry_table.go",    // the metric SSOT
		"go/internal/guards/role.go",                    // the guard itself
		"knowledge-base/research/flag-campaign-plan.json",
		"skills/audit/SKILL.md",
		".claude/settings.json",
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

// TestRole_AllowsLegitWorktreeWritesUnderProtection confirms the boundary does
// not over-block: ordinary source and a cycle's OWN go/acs/cycleN predicates
// remain writable in the build phase.
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
		"go/internal/core/orchestrator.go",  // ordinary source
		"go/acs/cycle20/predicates_test.go", // the cycle's OWN predicates (NOT under regression/)
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

// TestRole_OutsideCycleAllowsControlPlane confirms the human operator (outside any
// cycle, via `evolve ship --class manual`) can still edit the control plane —
// the boundary applies to autonomous cycles, not operator-driven changes.
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

// TestRole_DeniesGlobalSettingsInCycle is the C1 regression: the global
// ~/.claude/settings.json (which wires the PreToolUse hooks) matches the legacy
// "always-safe $HOME/.claude" rule, but a cycle must NOT be able to disable the
// guards by rewriting it — the integrity check takes precedence inside a cycle.
func TestRole_DeniesGlobalSettingsInCycle(t *testing.T) {
	s, _ := setupStorageWithCS(t, core.CycleState{
		CycleID: 30, Phase: "build", ActiveAgent: "builder",
		ActiveWorktree: "/work/wt/cycle-30",
		WorkspacePath:  filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-30"),
	})
	g := NewRole(s, false)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": filepath.Join(homeDir(), ".claude/settings.json")},
	})
	if dec.Allow {
		t.Error("a cycle must NOT rewrite the global ~/.claude/settings.json hook wiring")
	}
	if !dec.Alarm {
		t.Error("global settings.json deny must raise an Alarm")
	}
}

// TestRole_AllowsGlobalSettingsOutsideCycle confirms the operator can still edit
// their own ~/.claude/settings.json outside a cycle (the C1 fix must not break it).
func TestRole_AllowsGlobalSettingsOutsideCycle(t *testing.T) {
	s, _ := setupStorageNoCS(t)
	g := NewRole(s, false)
	dec := g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": filepath.Join(homeDir(), ".claude/settings.json")},
	})
	if !dec.Allow {
		t.Errorf("operator (no active cycle) must be able to edit global settings: %s", dec.Reason)
	}
}

// TestRole_BypassProtectedPathAlarms is H2: even an emergency --bypass of a
// protected control-plane path is allowed but ALARMED — never silent.
func TestRole_BypassProtectedPathAlarms(t *testing.T) {
	g := NewRole(nil, true) // bypass=true
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
	// A non-protected bypass must allow WITHOUT an alarm.
	dec = g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Edit",
		ToolInput: map[string]any{"file_path": "/repo/go/internal/core/foo.go"},
	})
	if !dec.Allow || dec.Alarm {
		t.Errorf("non-protected bypass must allow without alarm (allow=%v alarm=%v)", dec.Allow, dec.Alarm)
	}
}

package guards

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// Role enforces per-phase write allowlists for Edit/Write tools.
// Phase-1 subset of scripts/guards/role-gate.sh rules:
//   - build:        workspace_path + active_worktree
//   - audit:        workspace_path only (audit-*.{md,json} go there)
//   - learn/retrospective: workspace_path + .evolve/lessons/**
//   - other phases: workspace_path only
//   - Always-safe:  /tmp/**, $HOME/.claude/**
type Role struct {
	storage core.Storage
	bypass  bool
}

func NewRole(s core.Storage, bypass bool) *Role { return &Role{storage: s, bypass: bypass} }

func (r *Role) Name() string { return "role" }

func (r *Role) Decide(ctx context.Context, in core.GuardInput) core.GuardDecision {
	if in.ToolName != "Edit" && in.ToolName != "Write" {
		return core.GuardDecision{Allow: true}
	}
	path := strField(in, "file_path")
	if path == "" {
		return core.GuardDecision{Allow: true}
	}
	if r.bypass {
		// Even an emergency --bypass of the control plane is alarmed, never silent.
		if IsProtectedSurface(path) {
			return core.GuardDecision{Allow: true, Alarm: true,
				Reason: "role guard --bypass of a protected control-plane path: " + path}
		}
		return core.GuardDecision{Allow: true}
	}
	// Always-safe scratch dirs — EXCEPT a protected control-plane path that lives
	// there (e.g. the global ~/.claude/settings.json hook wiring), which must still
	// face the integrity check below.
	if isAlwaysSafe(path) && !IsProtectedSurface(path) {
		return core.GuardDecision{Allow: true}
	}
	if r.storage == nil {
		return core.GuardDecision{
			Allow:  false,
			Reason: "role guard: storage not configured; refusing Edit/Write by default",
		}
	}
	cs, err := r.storage.ReadCycleState(ctx)
	if err != nil {
		return core.GuardDecision{Allow: false, Reason: "role guard: cycle-state read failed: " + err.Error()}
	}
	// Outside an active cycle, allow (operator-driven changes via
	// `evolve ship --class manual` legitimately edit anything, incl. the control
	// plane below).
	if cs.CycleID == 0 {
		return core.GuardDecision{Allow: true}
	}
	// INTEGRITY BOUNDARY (control-plane sandbox, ADR-0064): no phase may modify the
	// gate/metric/guard/contract that grades its own cycle. Overrides the
	// worktree/workspace allowances below and applies to every phase; a hit is
	// denied AND alarmed. See guards.IsProtectedSurface for the surface + rationale.
	if IsProtectedSurface(path) {
		return core.GuardDecision{
			Allow: false,
			Alarm: true,
			Reason: "INTEGRITY VIOLATION (control-plane boundary): phase=" + cs.Phase +
				" attempted to modify the pipeline control plane (path=" + path +
				"). The gate/metric/guard/contract that grades a cycle may not be edited by that cycle; " +
				"control-plane changes require human-gated `evolve ship --class manual` outside a cycle.",
		}
	}
	if isUnderDir(path, cs.WorkspacePath) {
		return core.GuardDecision{Allow: true}
	}
	if core.WorktreePhase(core.Phase(cs.Phase)) && cs.ActiveWorktree != "" && isUnderDir(path, cs.ActiveWorktree) {
		return core.GuardDecision{Allow: true}
	}
	return core.GuardDecision{
		Allow: false,
		Reason: "role guard: phase=" + cs.Phase + " may not write outside workspace " +
			cs.WorkspacePath + " (path=" + path + "); pass --bypass to override in an emergency",
	}
}

func isAlwaysSafe(path string) bool {
	if strings.HasPrefix(path, "/tmp/") || path == "/tmp" {
		return true
	}
	if h := os.Getenv("HOME"); h != "" && strings.HasPrefix(path, filepath.Join(h, ".claude")+"/") {
		return true
	}
	return false
}

func isUnderDir(path, dir string) bool {
	if dir == "" {
		return false
	}
	if !filepath.IsAbs(dir) {
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
	}
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	if rel == "." || rel == "" {
		return true
	}
	return !strings.HasPrefix(rel, "..")
}

package guards

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
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
	if r.bypass {
		return core.GuardDecision{Allow: true}
	}
	if in.ToolName != "Edit" && in.ToolName != "Write" {
		return core.GuardDecision{Allow: true}
	}
	path := strField(in, "file_path")
	if path == "" {
		return core.GuardDecision{Allow: true}
	}
	if isAlwaysSafe(path) {
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
	// Outside an active cycle, allow.
	if cs.CycleID == 0 {
		return core.GuardDecision{Allow: true}
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

package guards

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type Role struct {
	storage core.Storage
	bypass  bool
	home    string
}

func NewRole(s core.Storage, bypass bool) *Role {
	return newRoleWithHome(s, bypass, os.Getenv("HOME"))
}

func newRoleWithHome(s core.Storage, bypass bool, home string) *Role {
	return &Role{storage: s, bypass: bypass, home: home}
}

func (r *Role) Name() string { return "role" }

func (r *Role) Decide(ctx context.Context, in core.GuardInput) core.GuardDecision {
	if in.ToolName != "Edit" && in.ToolName != "Write" {
		return core.GuardDecision{Allow: true}
	}
	path := strField(in, "file_path")
	if path == "" {
		return core.GuardDecision{Allow: true}
	}
	path = filepath.Clean(path)
	if r.bypass {
		if IsProtectedSurface(path) {
			return core.GuardDecision{Allow: true, Alarm: true,
				Reason: "role guard --bypass of a protected control-plane path: " + path}
		}
		return core.GuardDecision{Allow: true}
	}
	if isAlwaysSafe(path, r.home) && !IsProtectedSurface(path) {
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
	if cs.CycleID == 0 {
		return core.GuardDecision{Allow: true}
	}
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
	if cs.Phase == "retro" && isLessonsCorpusPath(path) {
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

func isLessonsCorpusPath(path string) bool {
	p := strings.ToLower(filepath.ToSlash(path))
	return strings.Contains(p, "/.evolve/instincts/lessons/")
}

func isAlwaysSafe(path, home string) bool {
	if strings.HasPrefix(path, "/tmp/") || path == "/tmp" {
		return true
	}
	if home == "" {
		return false
	}
	return strings.HasPrefix(path, filepath.Join(home, ".claude")+"/")
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

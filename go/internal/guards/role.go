package guards

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

type Role struct {
	storage core.Storage
	bypass  bool
	home    string
	now     func() time.Time
	alive   func(int) bool
}

func NewRole(s core.Storage, bypass bool) *Role {
	return newRoleWithHome(s, bypass, os.Getenv("HOME"))
}

func newRoleWithHome(s core.Storage, bypass bool, home string) *Role {
	return &Role{storage: s, bypass: bypass, home: home, now: time.Now, alive: runlease.PIDAlive}
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
	active, leaseErr := r.cycleActive(cs)
	if !active {
		return core.GuardDecision{Allow: true}
	}
	return decideInCycle(cs, path, leaseErr)
}

func decideInCycle(cs core.CycleState, path string, leaseErr error) core.GuardDecision {
	note := leaseNote(leaseErr)
	if IsProtectedSurface(path) {
		return core.GuardDecision{
			Allow: false,
			Alarm: true,
			Reason: "INTEGRITY VIOLATION (control-plane boundary): phase=" + cs.Phase +
				" attempted to modify the pipeline control plane (path=" + path +
				"). The gate/metric/guard/contract that grades a cycle may not be edited by that cycle; " +
				"control-plane changes require human-gated `evolve ship --class manual` outside a cycle." + note,
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
			cs.WorkspacePath + " (path=" + path + "); pass --bypass to override in an emergency" + note,
	}
}

func leaseNote(err error) string {
	if err == nil {
		return ""
	}
	return "; the run lease could not be read, so the owner counts as live: " + err.Error()
}

func (r *Role) cycleActive(cs core.CycleState) (bool, error) {
	if cs.CycleID == 0 {
		return false, nil
	}
	if !core.IsTerminalPhase(cs.Phase) {
		return true, nil
	}
	if cs.WorkspacePath == "" {
		return false, nil
	}
	lease, ok, err := runlease.Read(cs.WorkspacePath)
	if err != nil {
		return true, err
	}
	return ok && runlease.OwnerLive(lease, r.now(), runlease.DefaultTTL, r.alive), nil
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

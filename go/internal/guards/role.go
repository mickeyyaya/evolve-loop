package guards

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Role enforces per-phase Edit/Write allowlists and denies, with an alarm, any in-cycle write to the control plane.
type Role struct {
	storage core.Storage
	bypass  bool
	// home is the operator home of the always-safe <home>/.claude rule, fixed at construction.
	// Empty disables that rule; it never degrades to a relative ".claude/".
	home string
}

// NewRole returns a Role guard whose always-safe home is $HOME, read once here.
func NewRole(s core.Storage, bypass bool) *Role {
	return newRoleWithHome(s, bypass, os.Getenv("HOME"))
}

// newRoleWithHome keeps an empty home empty: a fabricated fallback such as /tmp would turn the
// <home>/.claude rule into the /tmp rule.
func newRoleWithHome(s core.Storage, bypass bool, home string) *Role {
	return &Role{storage: s, bypass: bypass, home: home}
}

// Name reports "role".
func (r *Role) Name() string { return "role" }

// Decide allows an Edit or Write inside the current phase's allowlist and fails closed when cycle state cannot be read.
func (r *Role) Decide(ctx context.Context, in core.GuardInput) core.GuardDecision {
	if in.ToolName != "Edit" && in.ToolName != "Write" {
		return core.GuardDecision{Allow: true}
	}
	path := strField(in, "file_path")
	if path == "" {
		return core.GuardDecision{Allow: true}
	}
	// Every decision below judges the path the write lands on: .. and . are resolved once, here.
	path = filepath.Clean(path)
	if r.bypass {
		// Even an emergency --bypass of the control plane is alarmed, never silent.
		if IsProtectedSurface(path) {
			return core.GuardDecision{Allow: true, Alarm: true,
				Reason: "role guard --bypass of a protected control-plane path: " + path}
		}
		return core.GuardDecision{Allow: true}
	}
	// A protected path inside an always-safe dir (the global ~/.claude/settings.json hook wiring)
	// still faces the integrity check below.
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
	// Outside a cycle the operator may edit anything, the control plane included.
	if cs.CycleID == 0 {
		return core.GuardDecision{Allow: true}
	}
	// No phase may edit the control plane that grades its own cycle. This check precedes every
	// allowance below, applies to every phase, and alarms.
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
	// Retro persists failure lessons to the durable corpus. This sits after the control-plane
	// boundary so no protected path can ride it.
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

// isLessonsCorpusPath reports whether path is under the lesson corpus .evolve/instincts/lessons/,
// matched as a fragment like IsProtectedSurface because the guard knows no project root.
func isLessonsCorpusPath(path string) bool {
	p := strings.ToLower(filepath.ToSlash(path))
	return strings.Contains(p, "/.evolve/instincts/lessons/")
}

// isAlwaysSafe reports whether path is scratch space every phase may write. An empty home disables
// the <home>/.claude rule, since filepath.Join("", ".claude") would bless every relative ".claude/" path.
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

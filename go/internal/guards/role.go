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
//   - learn/retrospective: workspace_path + .evolve/instincts/lessons/**
//   - other phases: workspace_path only
//   - Always-safe:  /tmp/**, <home>/.claude/**
type Role struct {
	storage core.Storage
	bypass  bool
	// home is the operator home the always-safe <home>/.claude rule resolves
	// against, captured ONCE at construction. Injected rather than read inside
	// Decide so (a) one process makes one reproducible decision instead of
	// re-reading a mutable env per tool call, and (b) tests grade a hermetic
	// fixture home instead of whatever machine they run on — the wound that let
	// the C1 global-settings regression assert a HOST-dependent path, and
	// /tmp/.claude/... on a HOME-less runner (guards-role-hermetic-home).
	// Empty means "no home resolved": the <home>/.claude rule then matches
	// nothing at all, never a bare relative ".claude/".
	home string
}

// NewRole builds the role guard for the current process, resolving the operator
// home from $HOME. This is the composition root's constructor
// (internal/cli/guardcmd.buildGuard, behind `evolve guard role`); the internal
// newRoleWithHome seam exists for hermetic tests.
func NewRole(s core.Storage, bypass bool) *Role {
	return newRoleWithHome(s, bypass, os.Getenv("HOME"))
}

// newRoleWithHome is the home-injection seam. An unresolvable/absent home is
// passed through as "" — never defaulted to a real directory, since a fabricated
// fallback (the previous test helper's "/tmp") silently converts the
// <home>/.claude rule into the /tmp rule.
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
	// Retro lessons allowance — documented in this file's header since phase-1
	// but implemented only now (cycles 1036/1041/1042 pinned the doc-only gap):
	// the retrospective phase persists failure-lesson YAMLs to the durable
	// corpus. Path-fragment anchored (the guard carries no project root; the
	// fragment idiom mirrors IsProtectedSurface) and phase-scoped, AFTER the
	// control-plane boundary above so no protected path can ride it.
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

// isLessonsCorpusPath reports whether path targets the durable failure-lesson
// corpus (.evolve/instincts/lessons/**), slash-normalized and case-folded like
// IsProtectedSurface.
func isLessonsCorpusPath(path string) bool {
	p := strings.ToLower(filepath.ToSlash(path))
	return strings.Contains(p, "/.evolve/instincts/lessons/")
}

// isAlwaysSafe reports whether path is scratch space every phase may write.
// Pure: home is the caller's resolved operator home (Role.home) — no env read,
// no filesystem access. An empty home disables the <home>/.claude rule entirely
// rather than degrading to filepath.Join("", ".claude") == ".claude", which would
// bless any relative ".claude/..." path on a HOME-less runner.
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

// Package runscope is the single source of truth for every run-, cycle-, and
// lane-scoped name the concurrent evolve-loop emits: git worktree branches and
// directories (global to one shared .git object store) and tmux session prefixes
// (global to one tmux server). Centralizing them in one immutable Value Object
// retires the duplicated branch-minting sites (core/swarm/cmd) and the parallel
// token schemes that previously drifted.
//
// It is the COMPOSER of two existing leaf primitives — it does not re-implement
// either rule:
//   - the LANE (worktree namespace) is internal/projecthash's 8-hex SHA256;
//   - the RUN token (session namespace) is internal/sessionrecord.RunScopeToken.
//
// Both of those stay leaf packages with a single home; runscope depends on them
// (never the reverse), so core, swarm, cmd/evolve, and internal/bridge can all
// import runscope with no import cycle (notably: swarm gains no direct core
// dependency, and sessionrecord stays the leaf the observer/bridge share).
//
// Naming model. The only Layer-1 collision surface is the git branch / worktree
// directory namespace, which is GLOBAL across the sibling worktrees that share
// one object store. The discriminator is the LANE: a stable-per-worktree
// identity that survives resume (so a resumed cycle reuses its branch + warm
// worktree). The per-invocation ULID RunID is deliberately NOT in any path — it
// lives only in the ledger, the run.json sidecar, and tmux session names. This
// is the GitLab CI_CONCURRENT_ID / Jenkins @N / Temporal pattern: stable path +
// bounded lane + run-id in a sidecar (ID-in-path breaks resume + warm-cache).
package runscope

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/projecthash"
	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
)

// EnvLane is the environment variable an operator sets to pin a human-readable
// lane for a worktree (e.g. EVOLVE_LANE=campaign), overriding the hash-of-root
// default. Readability only — correctness never depends on it, since the hash
// default is already collision-safe across distinct roots.
const EnvLane = "EVOLVE_LANE"

// Lane is the stable-per-worktree discriminator embedded in cycle worktree
// branch and directory names. Distinct concurrent roots yield distinct lanes
// (never collide); one root yields the same lane across resume (idempotent reuse
// of branch + worktree). It is always a git-ref-safe string.
type Lane string

// RunScope is the immutable Value Object carrying the three coordinates that name
// everything in a cycle: the stable Lane, the per-invocation RunID (a ULID; may
// be empty in lane+cycle-only contexts such as the `evolve worktree` CLI), and
// the Cycle number. Every name is a pure projection of these fields — two equal
// RunScopes produce byte-identical names.
type RunScope struct {
	lane  Lane
	runID string
	cycle int
}

// New builds a RunScope from an already-resolved lane, a RunID ("" when there is
// no run, e.g. the stateless worktree CLI), and a cycle number. Resolve the lane
// via ResolveLane (or LaneFromRoot) so the resolution strategy lives in one place.
func New(lane Lane, runID string, cycle int) RunScope {
	return RunScope{lane: lane, runID: runID, cycle: cycle}
}

// ResolveLane is the lane-resolution strategy. Precedence: an explicit value
// (e.g. a --lane flag) wins; else EVOLVE_LANE from getenv; else the hash-of-root
// default (LaneFromRoot). Explicit and env values are sanitized to a git-ref-safe
// token, and an all-unsafe override falls through to the next source. getenv may
// be nil (treated as unset).
func ResolveLane(explicit, root string, getenv func(string) string) Lane {
	if s := sanitizeLane(explicit); s != "" {
		return Lane(s)
	}
	if getenv != nil {
		if s := sanitizeLane(getenv(EnvLane)); s != "" {
			return Lane(s)
		}
	}
	return LaneFromRoot(root)
}

// LaneFromRoot is the zero-config default lane: projecthash.ForProjectRoot over
// the absolute, cleaned root. It is a pure function of the root, so distinct
// roots differ and one root is stable across resume; path variants of one dir
// (trailing slash, relative form) normalize to the same lane. The root is
// absolutized+cleaned BEFORE hashing, matching the shipped hotfix token exactly,
// so existing cycle-<lane>-N worktrees stay valid.
func LaneFromRoot(root string) Lane {
	return Lane(projecthash.ForProjectRoot(absClean(root)))
}

// LaneToken returns the stable git-ref-safe lane discriminator.
func (s RunScope) LaneToken() string { return string(s.lane) }

// Cycle returns the cycle number.
func (s RunScope) Cycle() int { return s.cycle }

// RunID returns the per-invocation ULID ("" if unset).
func (s RunScope) RunID() string { return s.runID }

// CycleBranch returns the per-cycle worktree branch / directory leaf name
// "cycle-<lane>-<N>", used for BOTH the -B branch and the directory leaf so the
// in-process provisioner and the `evolve worktree` CLI agree.
func (s RunScope) CycleBranch() string {
	return "cycle-" + string(s.lane) + "-" + strconv.Itoa(s.cycle)
}

// WorktreeDir joins base with CycleBranch(): <base>/cycle-<lane>-<N>.
func (s RunScope) WorktreeDir(base string) string {
	return filepath.Join(base, s.CycleBranch())
}

// IntegrationBranch returns the swarm shared-integration branch/dir leaf
// "cycle-<lane>-<N>-integration".
func (s RunScope) IntegrationBranch() string {
	return s.CycleBranch() + "-integration"
}

// WorkerBranch returns a swarm worker branch/dir leaf "cycle-<lane>-<N>-<workerID>".
func (s RunScope) WorkerBranch(workerID string) string {
	return s.CycleBranch() + "-" + workerID
}

// WorkspacePath returns the STABLE, token-FREE run-workspace path
// <root>/.evolve/runs/cycle-<N>. It must NOT embed the lane: state is physically
// isolated per worktree's own .evolve/, and the CB.4 run.json symlink + CB.5
// tmux registry path are matched on the bare cycle-<N> form across the worktree
// boundary. Mirrors core.RunWorkspacePath; provided here so callers have one
// naming entry point.
func (s RunScope) WorkspacePath(root string) string {
	return filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(s.cycle))
}

// RunToken returns the session-name run namespace ("r"+runID[:8]), delegating to
// sessionrecord.RunScopeToken so it stays byte-identical to the token the bridge
// mints and the observer (CB.6) asserts.
func (s RunScope) RunToken() string {
	return sessionrecord.RunScopeToken(s.runID)
}

// SessionPrefix returns the run-scoped infix the bridge injects after the driver
// prefix: RunToken()+"-" when RunID is set, else "" — byte-identical to the
// pre-CB.5 legacy session name for an empty RunID.
func (s RunScope) SessionPrefix() string {
	if s.runID == "" {
		return ""
	}
	return s.RunToken() + "-"
}

// absClean returns the absolute, cleaned form of root (best-effort: an Abs error,
// possible only when the cwd is unreadable, falls back to the cleaned input). It
// matches the shipped hotfix's pre-hash normalization so LaneFromRoot reproduces
// the existing worktree tokens.
func absClean(root string) string {
	c := filepath.Clean(root)
	if abs, err := filepath.Abs(c); err == nil {
		return abs
	}
	return c
}

// sanitizeLane strips an explicit/env lane override to a git-ref-safe token:
// [A-Za-z0-9_-] only, no leading/trailing '-'. It deliberately drops '.' so an
// override can never introduce the git-ref-illegal forms a dot enables ("..",
// a trailing ".lock", a leading "."); lanes are short identifiers that do not
// need dots. Returns "" when nothing safe remains so ResolveLane falls through.
func sanitizeLane(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-")
}

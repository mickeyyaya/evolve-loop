package swarm

import "context"

// Launcher is the narrow seam the dispatcher needs from the bridge: launch one
// worker and report its exit/cost/session-identity. It mirrors core.Bridge.Launch
// but is defined here (accept-interface-where-used) so the swarm package does not
// import core for dispatch — the composition root adapts core.Bridge to this.
type Launcher interface {
	Launch(ctx context.Context, req LaunchRequest) (LaunchResult, error)
}

// LaunchRequest is the per-worker launch contract — the subset of
// core.BridgeRequest the dispatcher fills. The composition root maps this onto
// core.BridgeRequest 1:1.
type LaunchRequest struct {
	CLI          string
	Model        string
	Profile      string
	Agent        string // "<task-or-mode>-w<i>" — collision-safe tmux/inbox key
	SessionName  string // deterministic tmux session name (orphan-on-cancel hardening); empty for headless
	Prompt       string
	Workspace    string
	Worktree     string
	ProjectRoot  string
	ArtifactPath string
	Cycle        int
	// Env is the per-worker environment overlay the dispatcher computes (e.g. an
	// isolated PORT for writer dev servers). The composition root merges it OVER
	// the shared phase env, so per-worker keys win. Nil/empty = phase env only.
	Env map[string]string
}

// LaunchResult is the per-worker launch outcome (subset of core.BridgeResponse).
type LaunchResult struct {
	ExitCode int
	CostUSD  float64
	// PGID is the launched process group (0 if the launcher can't report one);
	// the dispatcher records it so the reaper can group-kill.
	PGID int
	// TmuxSession is the session name the driver used (empty for headless).
	TmuxSession string
}

// Deps are the injected ports for a swarm dispatch — Dependency Injection so the
// dispatcher is unit-testable with fakes and carries no hidden global state.
type Deps struct {
	Launcher    Launcher
	Provisioner WorkerProvisioner // writers only (readers pass nil)
	Killer      SessionKiller     // optional; nil skips per-session reap
	Registry    *SessionRegistry  // optional; nil skips session tracking
	// Concurrency caps how many workers launch at once (semaphore size). <=0
	// means "one slot per worker" (unbounded); callers should pass
	// EVOLVE_SWARM_CONCURRENCY (default 2).
	Concurrency int
	// PortBase is the first port handed to writer workers (worker i → base+i) so
	// their dev servers don't collide. <=0 falls back to DefaultPortBase.
	PortBase int
}

package swarm

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// Launcher launches one worker and reports its exit, cost and session identity.
type Launcher interface {
	Launch(ctx context.Context, req LaunchRequest) (LaunchResult, error)
}

// LaunchRequest is the per-worker launch contract, mapped 1:1 onto core.BridgeRequest by the composition root.
type LaunchRequest struct {
	CLI          string
	Model        string
	Profile      string
	Agent        string // "<task-or-mode>-<workerID>": the collision-safe tmux/inbox key
	SessionName  string // deterministic tmux session name; empty for headless
	Prompt       string
	Workspace    string
	Worktree     string
	ProjectRoot  string
	ArtifactPath string
	Cycle        int
	// Env is merged over the shared phase env, so per-worker keys win; nil means the phase env only.
	Env map[string]string
}

// LaunchResult is the per-worker launch outcome, a subset of core.BridgeResponse.
type LaunchResult struct {
	ExitCode    int
	CostUSD     float64
	Tokens      cyclestate.TokenUsage
	PGID        int    // 0 when the launcher cannot report one
	TmuxSession string // empty for headless
}

// Deps are the injected ports for one swarm dispatch.
type Deps struct {
	Launcher    Launcher
	Provisioner WorkerProvisioner // writers only; readers pass nil
	Killer      SessionKiller     // nil skips per-session reap
	Registry    *SessionRegistry  // nil skips session tracking
	// Concurrency caps simultaneous launches; <=0 means one slot per worker.
	Concurrency int
	// PortBase is writer worker 0's port (worker i gets base+i); <=0 means DefaultPortBase.
	PortBase int
}

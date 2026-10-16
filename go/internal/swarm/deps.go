package swarm

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

type Launcher interface {
	Launch(ctx context.Context, req LaunchRequest) (LaunchResult, error)
}

type LaunchRequest struct {
	CLI          string
	Model        string
	Profile      string
	Agent        string
	SessionName  string
	Prompt       string
	Workspace    string
	Worktree     string
	ProjectRoot  string
	ArtifactPath string
	Cycle        int
	Env          map[string]string
}

type LaunchResult struct {
	ExitCode    int
	CostUSD     float64
	Tokens      cyclestate.TokenUsage
	PGID        int
	TmuxSession string
}

type Deps struct {
	Launcher    Launcher
	Provisioner WorkerProvisioner
	Killer      SessionKiller
	Registry    *SessionRegistry
	Concurrency int
	PortBase    int
}

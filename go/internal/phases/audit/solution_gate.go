package audit

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func solutionGate(spec config.DeliverableKindSpec) func(req core.PhaseRequest) ([]string, error) {
	return func(req core.PhaseRequest) ([]string, error) {
		return core.SolutionViolations(req.Workspace, req.Worktree, req.ProjectRoot, spec), nil
	}
}

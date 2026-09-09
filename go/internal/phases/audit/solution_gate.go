package audit

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// solutionGate is the production solution-contract gate (ADR-0099 slice 2):
// for a document cycle it reports every contract violation the ONE engine
// finds over the bound tasks' deliverables — the same core.SolutionViolations
// the build floor runs, over the same registry spec the composition root hands
// both of them, so the two surfaces cannot disagree. Code cycles report
// nothing. The gate never loads config itself: the root owns that policy.
func solutionGate(spec config.DeliverableKindSpec) func(req core.PhaseRequest) ([]string, error) {
	return func(req core.PhaseRequest) ([]string, error) {
		return core.SolutionViolations(req.Workspace, req.Worktree, req.ProjectRoot, spec), nil
	}
}

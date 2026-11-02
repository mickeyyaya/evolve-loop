package auditfixports

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/auditfix"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type Gates struct {
	Floor       core.BuildHandoffFloor
	Protected   func(string) bool
	ProjectRoot string
	ACS         func(acssuite.Options) (acssuite.Verdict, error)
}

var _ auditfix.Gates = Gates{}

func (g Gates) Run(ctx context.Context, req auditfix.GateRequest) (auditfix.Verdict, error) {
	cs, err := core.ReadRunCycleState(req.Workspace, "")
	if err != nil {
		return auditfix.Verdict{}, fmt.Errorf("auditfixports: gates: %w", err)
	}
	in := core.ReviewInputFor(cs, core.PhaseBuild, g.ProjectRoot)
	in.Cycle, in.Workspace, in.Worktree, in.Protected = req.Cycle, req.Workspace, req.Worktree, g.Protected
	reasons := g.Floor.Failures(ctx, in)
	acs, err := g.ACS(acssuite.Options{Root: req.Worktree, ProjectRoot: g.ProjectRoot, Cycle: req.Cycle})
	if err != nil {
		return auditfix.Verdict{}, fmt.Errorf("auditfixports: acs: %w", err)
	}
	for _, id := range acs.RedIDs {
		reasons = append(reasons, "acs red: "+id)
	}
	return auditfix.Verdict{Pass: len(reasons) == 0, Reasons: reasons}, nil
}

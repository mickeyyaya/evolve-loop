package routingtest

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

type scriptedProposer struct {
	spec      AgentSpec
	calls     int
	planCalls int
	seen      []string
}

func (p *scriptedProposer) Propose(in router.RouteInput) (*router.Proposal, error) {
	p.calls++
	p.seen = append(p.seen, in.Current)
	if p.spec.ErrorsOn[in.Current] {
		return nil, fmt.Errorf("scripted proposer degrade at %s", in.Current)
	}
	return p.spec.Proposals[in.Current], nil
}

func (p *scriptedProposer) Plan(router.RouteInput) (*router.PhasePlan, error) {
	p.planCalls++
	if p.spec.PlanError {
		return nil, fmt.Errorf("scripted planner degrade")
	}
	return &router.PhasePlan{Entries: append([]router.PhasePlanEntry(nil), p.spec.Plan...)}, nil
}

var (
	_ router.Proposer = (*scriptedProposer)(nil)
	_ router.Planner  = (*scriptedProposer)(nil)
)

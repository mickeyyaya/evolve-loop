package routingtest

import (
	"fmt"

	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

// scriptedProposer is the deterministic stand-in for the LLM router brain. Keyed
// by the just-completed phase (RouteInput.Current), it returns a canned proposal
// or a forced error (to exercise LLMProposal's degrade-to-static path). A nil
// proposal for a phase is legal — the kernel decision stands unchanged.
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

// Plan implements router.Planner: it returns the scripted upfront whole-cycle
// plan, or a forced error to exercise the orchestrator's degrade-to-static-spine
// fail-safe. The returned plan is UNCLAMPED — the caller (orchestrator or the
// pure engine) runs ClampPlanToFloor, exactly as production does.
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

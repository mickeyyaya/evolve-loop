package router

import "github.com/mickeyyaya/evolve-loop/go/internal/config"

// RoutingStrategy is the routing brain, selected once at the composition root; every implementation ends in Route.
type RoutingStrategy interface {
	Decide(in RouteInput) RouterDecision

	// Recover returns the recovery route for a ship-failure Blocker; implementations delegate to Recover.
	Recover(in RouteInput) RouterDecision
}

// StaticPreset is the deterministic brain: triggers and spine, no LLM.
type StaticPreset struct{}

// Decide routes without a proposal.
func (StaticPreset) Decide(in RouteInput) RouterDecision { return Route(in, nil) }

// Recover delegates to the shared recovery chain.
func (StaticPreset) Recover(in RouteInput) RouterDecision { return Recover(in) }

// Proposer produces the per-transition advisory proposal; core implements it over the bridge.
type Proposer interface {
	Propose(in RouteInput) (*Proposal, error)
}

// Planner produces the whole-cycle advisory plan once at cycle start; core implements it.
type Planner interface {
	Plan(in RouteInput) (*PhasePlan, error)
}

// LLMProposal asks a Proposer for advice, then routes through Route; a nil or failed proposal leaves the static decision.
type LLMProposal struct {
	Proposer Proposer
}

// Decide consults the Proposer when shouldPropose allows, then routes.
func (s LLMProposal) Decide(in RouteInput) RouterDecision {
	var p *Proposal
	if s.Proposer != nil && shouldPropose(in) {
		if got, err := s.Proposer.Propose(in); err == nil {
			p = got
		}
	}
	return Route(in, p)
}

// Recover delegates to the shared recovery chain; recovery needs no LLM.
func (LLMProposal) Recover(in RouteInput) RouterDecision { return Recover(in) }

// shouldPropose implements the hybrid cadence: with a plan driving, only branch
// transitions reveal signals the plan could not foresee; with no plan, every transition proposes.
func shouldPropose(in RouteInput) bool {
	if in.Plan == nil {
		return true
	}
	return isBranchTransition(in.Current)
}

// isBranchTransition reports whether the completed phase produces new objective signals.
// Extend it when a phase starts producing such signals.
func isBranchTransition(current string) bool {
	switch normalize(current) {
	case "build", "audit", "retrospective":
		return true
	}
	return false
}

// Select returns LLMProposal for DynamicLLM mode with a proposer, else StaticPreset.
func Select(cfg config.RoutingConfig, proposer Proposer) RoutingStrategy {
	if cfg.Mode == config.ModeDynamicLLM && proposer != nil {
		return LLMProposal{Proposer: proposer}
	}
	return StaticPreset{}
}

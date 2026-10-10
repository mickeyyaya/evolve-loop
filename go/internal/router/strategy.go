package router

import "github.com/mickeyyaya/evolve-loop/go/internal/config"

type RoutingStrategy interface {
	Decide(in RouteInput) RouterDecision

	Recover(in RouteInput) RouterDecision
}

type StaticPreset struct{}

func (StaticPreset) Decide(in RouteInput) RouterDecision { return Route(in, nil) }

func (StaticPreset) Recover(in RouteInput) RouterDecision { return Recover(in) }

type Proposer interface {
	Propose(in RouteInput) (*Proposal, error)
}

type Planner interface {
	Plan(in RouteInput) (*PhasePlan, error)
}

type LLMProposal struct {
	Proposer Proposer
}

func (s LLMProposal) Decide(in RouteInput) RouterDecision {
	var p *Proposal
	if s.Proposer != nil && shouldPropose(in) {
		if got, err := s.Proposer.Propose(in); err == nil {
			p = got
		}
	}
	return Route(in, p)
}

func (LLMProposal) Recover(in RouteInput) RouterDecision { return Recover(in) }

func shouldPropose(in RouteInput) bool {
	if in.Plan == nil {
		return true
	}
	return isBranchTransition(in.Current)
}

func isBranchTransition(current string) bool {
	switch normalize(current) {
	case "build", "audit", "retrospective":
		return true
	}
	return false
}

func Select(cfg config.RoutingConfig, proposer Proposer) RoutingStrategy {
	if cfg.Mode == config.ModeDynamicLLM && proposer != nil {
		return LLMProposal{Proposer: proposer}
	}
	return StaticPreset{}
}

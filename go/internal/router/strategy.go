package router

import "github.com/mickeyyaya/evolve-loop/go/internal/config"

// RoutingStrategy is the routing brain (GoF Strategy). Both implementations
// converge on the same pure Route() clamp floor — the kernel is always the floor.
// Selecting a strategy ONCE at the composition root removes any `if mode==…`
// conditional from the orchestrator loop.
type RoutingStrategy interface {
	Decide(in RouteInput) RouterDecision
}

// StaticPreset is the deterministic brain: triggers + spine only, no LLM.
type StaticPreset struct{}

func (StaticPreset) Decide(in RouteInput) RouterDecision { return Route(in, nil) }

// Proposer produces an advisory routing proposal from the digested signals.
// The concrete implementation (which calls core.Bridge) lives in package core,
// keeping router a leaf — router defines only the interface it consumes.
type Proposer interface {
	Propose(in RouteInput) (*Proposal, error)
}

// LLMProposal is the dynamic-LLM brain: it asks a Proposer for advice, then
// defers to the same pure Route() clamp. A nil/failed proposal degrades cleanly
// to static behavior (the kernel decision stands).
type LLMProposal struct {
	Proposer Proposer
}

func (s LLMProposal) Decide(in RouteInput) RouterDecision {
	var p *Proposal
	if s.Proposer != nil {
		if got, err := s.Proposer.Propose(in); err == nil {
			p = got
		}
	}
	return Route(in, p)
}

// Select chooses the strategy from config. DynamicLLM requires a non-nil
// proposer; otherwise it falls back to the deterministic StaticPreset.
func Select(cfg config.RoutingConfig, proposer Proposer) RoutingStrategy {
	if cfg.Mode == config.ModeDynamicLLM && proposer != nil {
		return LLMProposal{Proposer: proposer}
	}
	return StaticPreset{}
}

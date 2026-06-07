package router

import "github.com/mickeyyaya/evolve-loop/go/internal/config"

// RoutingStrategy is the routing brain (GoF Strategy). Both implementations
// converge on the same pure Route() clamp floor — the kernel is always the floor.
// Selecting a strategy ONCE at the composition root removes any `if mode==…`
// conditional from the orchestrator loop.
type RoutingStrategy interface {
	Decide(in RouteInput) RouterDecision

	// Recover returns the recovery route for a ship-failure Blocker. It is
	// deterministic (Chain of Responsibility, no LLM) and shared across
	// strategies, so implementations delegate to the pure Recover function.
	Recover(in RouteInput) RouterDecision
}

// StaticPreset is the deterministic brain: triggers + spine only, no LLM.
type StaticPreset struct{}

func (StaticPreset) Decide(in RouteInput) RouterDecision { return Route(in, nil) }

// Recover implements RoutingStrategy for StaticPreset by delegating to the
// shared deterministic recovery chain.
func (StaticPreset) Recover(in RouteInput) RouterDecision { return Recover(in) }

// Proposer produces an advisory routing proposal from the digested signals.
// The concrete implementation (which calls core.Bridge) lives in package core,
// keeping router a leaf — router defines only the interface it consumes.
type Proposer interface {
	Propose(in RouteInput) (*Proposal, error)
}

// Planner produces the advisory WHOLE-CYCLE plan (ADR-0024 §2): a run/skip
// decision + rationale for every phase, computed once at cycle start (the cheap,
// coherent half of the hybrid cadence). Segregated from Proposer so a consumer
// that only needs per-transition advice need not depend on whole-cycle planning,
// and vice versa. Like Proposer, the concrete implementation lives in package
// core; the plan is advisory and the kernel clamp remains the floor.
type Planner interface {
	Plan(in RouteInput) (*PhasePlan, error)
}

// LLMProposal is the dynamic-LLM brain: it asks a Proposer for advice, then
// defers to the same pure Route() clamp. A nil/failed proposal degrades cleanly
// to static behavior (the kernel decision stands).
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

// Recover implements RoutingStrategy for LLMProposal by delegating to the
// shared deterministic recovery chain (recovery needs no LLM).
func (LLMProposal) Recover(in RouteInput) RouterDecision { return Recover(in) }

// shouldPropose implements the ADR-0024 §2 hybrid cadence. When an upfront
// whole-cycle plan is driving (in.Plan != nil — set only when the orchestrator
// produced a clamped plan, i.e. Stage>=Advisory + DynamicLLM), the per-transition
// Proposer adds value ONLY at BRANCH transitions — post-build and
// post-audit, where new objective signals (acs_red, audit verdict) appear that
// the signal-poor start-of-cycle plan could not foresee. Every other transition
// is already decided by the cached plan, and a proposal can never change the
// kernel's NextPhase anyway (see applyProposal — it only annotates/clamps), so
// calling the LLM there is wasted spend. With NO upfront plan (Shadow, static
// mode, or a planner failure) the legacy per-transition cadence stands, so
// Shadow-soak forensics are unchanged.
func shouldPropose(in RouteInput) bool {
	if in.Plan == nil {
		return true
	}
	return isBranchTransition(in.Current)
}

// isBranchTransition reports whether the just-completed phase is a routing branch
// point — where the verdict/signals genuinely fork the remaining plan. Extend
// this set when adding a phase that produces NEW post-phase objective signals
// worth a per-transition advisory (today: build's ACS, audit's verdict, and
// the retrospective's failure context — recovery retry/end + failure-scoped
// inserts are advisor-decidable, failure floor Phase 3).
func isBranchTransition(current string) bool {
	switch normalize(current) {
	case "build", "audit", "retrospective":
		return true
	}
	return false
}

// Select chooses the strategy from config. DynamicLLM requires a non-nil
// proposer; otherwise it falls back to the deterministic StaticPreset.
func Select(cfg config.RoutingConfig, proposer Proposer) RoutingStrategy {
	if cfg.Mode == config.ModeDynamicLLM && proposer != nil {
		return LLMProposal{Proposer: proposer}
	}
	return StaticPreset{}
}

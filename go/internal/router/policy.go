package router

import "github.com/mickeyyaya/evolve-loop/go/internal/config"

// PhasePolicy is the single enablement-decision object (Fowler's "decision point
// vs router"): it answers "should phase X run this cycle?" from the injected
// config + digested signals, so phase code never reads EVOLVE_* env vars itself.
// This collapses the scattered Skipper os.Getenv checks (triage/buildplanner/
// scout/audit/retro) into one polymorphic decision.
type PhasePolicy struct {
	Cfg config.RoutingConfig
}

// NewPhasePolicy builds a policy from the loaded config.
func NewPhasePolicy(cfg config.RoutingConfig) PhasePolicy { return PhasePolicy{Cfg: cfg} }

// Enabled reports whether a phase should run. Precedence mirrors the router's
// shouldRun (minus routing-level budget/insertion caps, which are not a phase's
// concern): mandatory > conditional-pin > forced on/off > content trigger.
func (p PhasePolicy) Enabled(phase string, sig RoutingSignals) bool {
	if isMandatory(p.Cfg, phase) {
		return true
	}
	if rule, ok := p.Cfg.Conditional[phase]; ok && evalCondRule(sig, rule) {
		return true
	}
	switch enableOf(p.Cfg, phase) {
	case config.EnableOff:
		return false
	case config.EnableOn:
		return true
	default:
		return triggerFires(sig, p.Cfg.Triggers[phase])
	}
}

package router

import (
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// PhasePolicy answers "should phase X run this cycle?" from injected config, so phase code never reads env vars.
type PhasePolicy struct {
	Cfg config.RoutingConfig
}

// NewPhasePolicy builds a policy from the loaded config.
func NewPhasePolicy(cfg config.RoutingConfig) PhasePolicy { return PhasePolicy{Cfg: cfg} }

// Enabled reports whether a phase should run, with shouldRun's precedence minus the insertion cap.
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

// ShouldRunPhase decides for a self-skipping phase. Below Enforce the static state machine owns
// sequencing, so it resolves only enable flags; from Enforce up it defers to Enabled, so a pinned
// phase cannot be flag-disabled.
func (p PhasePolicy) ShouldRunPhase(phase string) bool {
	if p.Cfg.Stage >= config.StageEnforce {
		return p.Enabled(phase, RoutingSignals{})
	}
	if isMandatory(p.Cfg, phase) {
		return true
	}
	switch enableOf(p.Cfg, phase) {
	case config.EnableOff:
		return false
	case config.EnableOn:
		return true
	default:
		return false
	}
}

// PolicyForProject loads the PhasePolicy for a phase from the registry and env; a missing registry uses defaults.
// Loading per phase avoids threading a policy through both phase-construction paths.
func PolicyForProject(projectRoot string, env map[string]string) PhasePolicy {
	cfg, _ := config.Load(config.RegistryPath(projectRoot), env)
	// Merge policy exactly as the composition root does, or a phase made mandatory only by
	// policy would skip itself. A malformed policy is ignored here and hard-fails at dispatch.
	if pol, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json")); err == nil {
		cfg.Mandatory = pol.MergeMandatory(cfg.Mandatory)
		cfg.AuditFailRoutesTo = FailureRouteFromPolicy(pol)
	}
	return PhasePolicy{Cfg: cfg}
}

// FailureRouteFromPolicy folds policy.json:failure_floor into the audit-FAIL route, or "" when absent.
// always_learn=false downgrades the default route to memo, but an explicit "retrospective" wins; the raw
// field is read because FailurePolicy launders defaults.
func FailureRouteFromPolicy(pol policy.Policy) string {
	if pol.FailureFloor == nil {
		return ""
	}
	alwaysLearn, route := pol.FailurePolicy()
	explicitRetro := pol.FailureFloor.AuditFailRoutesTo == "retrospective"
	if !alwaysLearn && route == "retrospective" && !explicitRetro {
		route = "memo"
	}
	return route
}

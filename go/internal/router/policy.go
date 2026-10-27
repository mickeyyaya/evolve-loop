package router

import (
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type PhasePolicy struct {
	Cfg config.RoutingConfig
}

func NewPhasePolicy(cfg config.RoutingConfig) PhasePolicy { return PhasePolicy{Cfg: cfg} }

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

func PolicyForProject(projectRoot string, env map[string]string, opts ...config.Option) PhasePolicy {
	cfg, _ := ConfigForProject(projectRoot, env, os.Stderr, opts...)
	return PhasePolicy{Cfg: cfg}
}

func ConfigForProject(projectRoot string, env map[string]string, warn io.Writer, opts ...config.Option) (config.RoutingConfig, []config.Warning) {
	loader := config.New(opts...)
	cfg, ws := loader.Load(config.RegistryPath(projectRoot), env)
	return ApplyPolicy(loader, cfg, policy.LoadOrDefaults(filepath.Join(projectRoot, ".evolve", "policy.json"), warn)), ws
}

func ApplyPolicy(loader *config.Loader, cfg config.RoutingConfig, pol policy.Policy) config.RoutingConfig {
	cfg, _ = loader.ApplyRoutingKnobs(cfg, RoutingKnobsOf(pol.GatesConfig(), pol.RouterConfig()))
	cfg.Mandatory = pol.MergeMandatory(cfg.Mandatory)
	cfg.AuditFailRoutesTo = FailureRouteFromPolicy(pol)
	return cfg
}

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

func RoutingKnobsOf(g policy.GatesConfig, rt policy.RouterPolicy) config.RoutingKnobs {
	return config.RoutingKnobs{
		DynamicRouting: rt.DynamicRouting, RoutingMode: rt.RoutingMode, ConditionalMandatory: rt.ConditionalMandatory,
		MaxOptionalInsertions: rt.MaxOptionalInsertions, PhaseIO: g.PhaseIO,
	}
}

package router

import (
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

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

// ShouldRunPhase is the enablement authority for a SELF-SKIPPING phase
// (triage/tdd/build-planner consult it from their Skipper.ShouldSkip). It is
// deliberately stage-aware:
//
//   - Below Enforce (Off/Shadow/Advisory) it is pure flag/enable resolution —
//     byte-identical to the legacy req.Env["EVOLVE_*"] checks — because the
//     STATIC state machine, not the router, owns sequencing there. The
//     conditional-pin and content triggers (which need digested signals) are
//     NOT applied at the phase level; signal-aware routing is the
//     orchestrator's job.
//   - From Enforce up it defers to the full kernel policy (Enabled), so a
//     conditional-pinned phase cannot be flag-disabled. Signal-aware
//     insert/skip already happened at the transition; the phase passes empty
//     signals here and simply confirms "run".
//
// This split is what lets the same one code path preserve legacy Stage:Off
// behavior while honoring the trust kernel under Enforce.
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
		// Optional phase with no explicit enable in a legacy stage ⇒ skip
		// (matches build-planner's opt-in default). With config defaults +
		// registry `enabled` set, the self-skipping phases never land here.
		return false
	}
}

// PolicyForProject resolves the PhasePolicy a phase should use from its
// request, via config.Load (the sole env interpreter — env is the injected
// req.Env map, never os.Getenv). A missing registry falls back to config
// defaults, so the legacy run/skip posture holds even in tests. Calling the
// deterministic loader per phase avoids threading the injected policy through
// the two phase-construction paths (orchestrator wiring + registry factories)
// and the zero-value-skip hazard that would create.
func PolicyForProject(projectRoot string, env map[string]string) PhasePolicy {
	registryPath := filepath.Join(projectRoot, "docs", "architecture", "phase-registry.json")
	cfg, _ := config.Load(registryPath, env)
	// Apply the user policy's mandatory_phases here too, identically to the
	// loop's composition root — otherwise a self-skipping phase (triage/tdd/
	// build-planner) made mandatory ONLY by policy would re-read config without
	// the merge and skip itself. A malformed policy is ignored here (best-effort
	// for the skip decision); the runner re-loads it and hard-fails at dispatch.
	if pol, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json")); err == nil {
		cfg.Mandatory = pol.MergeMandatory(cfg.Mandatory)
		cfg.AuditFailRoutesTo = FailureRouteFromPolicy(pol)
	}
	return PhasePolicy{Cfg: cfg}
}

// FailureRouteFromPolicy folds policy.json:failure_floor into the single
// audit-FAIL route the router consumes (Phase 4a — one user surface).
// always_learn=false tunes the DEFAULT route down to the lightweight memo
// phase — an explicitly written audit_fail_routes_to:"retrospective" wins
// (explicit beats derived; FailurePolicy launders defaults, so the raw
// field is checked here). An absent failure_floor returns "" so the
// deprecated enable-chain behavior stands for one more release. The
// deterministic floor is unaffected either way.
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

package phasespec

import "github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"

const CodeRoutingDefect signalcenter.Code = "CONFIG_PHASE_ROUTING_DEFECT"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleConfig, CodeRoutingDefect, "loading or routing the phase catalog found a defect that the cycle survives: a builtin registry that does not load, a user spec dropped, clamped or demoted (no persona), an invalid spec left unrouted, a minted phase that is rejected or clashes with a runner, or an anchor that does not resolve, so the phase is placed before audit; the reason is the warning verbatim and the origin names the composition step (cycle wiring or a mint)")
}

func RoutingDefectEvent(origin, warning string) signalcenter.Event {
	return signalcenter.Event{
		Module:   signalcenter.ModuleConfig,
		Origin:   origin,
		Kind:     signalcenter.KindConfigWarning,
		Code:     CodeRoutingDefect,
		Severity: signalcenter.SeverityWarn,
		Reason:   warning,
	}
}

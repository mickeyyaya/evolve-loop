package phasespec

import "github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"

const CodeRoutingDefect signalcenter.Code = "CONFIG_PHASE_ROUTING_DEFECT"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleConfig, CodeRoutingDefect, "loading or routing the phase catalog found a defect that the cycle survives: unloadable builtin registry, dropped/clamped/demoted user spec, or unrouted invalid spec. Also minted phases rejected or clashing, or unresolved anchors placed before audit (pre-build after build stays unrouted). Reason is warning verbatim; origin names step")
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

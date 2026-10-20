package phasespec

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestRoutingDefectEvent_IsTheRegisteredConfigWarning(t *testing.T) {
	e := RoutingDefectEvent("wireOrchestratorDeps", "phase late anchor early not in routing order — placed before audit")
	if e.Code != CodeRoutingDefect || e.Module != signalcenter.ModuleConfig || e.Kind != signalcenter.KindConfigWarning || e.Severity != signalcenter.SeverityWarn {
		t.Fatalf("a routing defect is the registered config WARN: %+v", e)
	}
	if e.Origin != "wireOrchestratorDeps" || e.Reason != "phase late anchor early not in routing order — placed before audit" {
		t.Fatalf("the event carries the caller's origin and the warning verbatim: %+v", e)
	}
	if m, ok := signalcenter.IsRegistered(CodeRoutingDefect); !ok || m != signalcenter.ModuleConfig {
		t.Fatalf("%s is not registered to the config module (module=%q)", CodeRoutingDefect, m)
	}
	if _, drift := signalcenter.Normalize(e); len(drift) != 0 {
		t.Fatalf("the Signal Center must accept the event without drift: %v", drift)
	}
}

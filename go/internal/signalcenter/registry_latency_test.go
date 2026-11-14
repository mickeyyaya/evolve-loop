package signalcenter

import "testing"

func TestRegistry_LatencyBudgetExceededBelongsToTheLatencyModule(t *testing.T) {
	t.Parallel()
	if string(ModuleLatency) != "latency" || !ModuleLatency.Known() {
		t.Fatalf("ModuleLatency = %q, known %v, want the closed module latency", ModuleLatency, ModuleLatency.Known())
	}
	owner, registered := IsRegistered(CodeLatencyBudgetExceeded)
	if !registered || owner != ModuleLatency || !CodeLatencyBudgetExceeded.BelongsTo(ModuleLatency) {
		t.Errorf("owner = %q, registered = %v, want %s registered by %s", owner, registered, CodeLatencyBudgetExceeded, ModuleLatency)
	}
	normalized, drift := Normalize(Event{Module: ModuleLatency, Origin: "Test.emit", Kind: KindLoopWarning, Severity: SeverityWarn, Code: CodeLatencyBudgetExceeded, Reason: "two waves breached"})
	if drift != nil || normalized.Code != CodeLatencyBudgetExceeded {
		t.Errorf("a registered WARN code must normalize without drift: %v %+v", drift, normalized)
	}
}

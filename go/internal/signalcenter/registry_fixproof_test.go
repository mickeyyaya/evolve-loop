package signalcenter

import "testing"

func TestRegistry_FixProofMissingBelongsToTheFixProofModule(t *testing.T) {
	t.Parallel()
	owner, registered := IsRegistered(CodeFixProofMissing)
	if !registered || owner != ModuleFixProof || !CodeFixProofMissing.BelongsTo(ModuleFixProof) {
		t.Fatalf("owner = %q, registered = %v, want %s registered by %s", owner, registered, CodeFixProofMissing, ModuleFixProof)
	}
	for _, c := range RegistryConflicts() {
		if c.Code == CodeFixProofMissing {
			t.Errorf("the code must register without a conflict: %+v", c)
		}
	}
	normalized, drift := Normalize(Event{Module: ModuleFixProof, Origin: "Test.emit", Kind: KindShipWarning, Severity: SeverityWarn, Code: CodeFixProofMissing, Reason: "no trailer"})
	if drift != nil || normalized.Code != CodeFixProofMissing {
		t.Errorf("a registered WARN code must normalize without drift: %v %+v", drift, normalized)
	}
}

package signalcenter

import "testing"

func TestRegistry_HostDiskLowBelongsToTheHostModule(t *testing.T) {
	t.Parallel()
	owner, registered := IsRegistered(CodeHostDiskLow)
	if !registered || owner != ModuleHost || !CodeHostDiskLow.BelongsTo(ModuleHost) {
		t.Fatalf("owner = %q, registered = %v, want %s registered by %s", owner, registered, CodeHostDiskLow, ModuleHost)
	}
	normalized, drift := Normalize(Event{Module: ModuleHost, Origin: "Test.emit", Kind: KindLoopWarning, Severity: SeverityWarn, Code: CodeHostDiskLow, Reason: "disk low"})
	if drift != nil || normalized.Code != CodeHostDiskLow {
		t.Errorf("a registered WARN code must normalize without drift: %v %+v", drift, normalized)
	}
}

func TestRegistry_HostVerifyLockWaitBelongsToTheHostModule(t *testing.T) {
	t.Parallel()
	owner, registered := IsRegistered(CodeHostVerifyLockWait)
	if !registered || owner != ModuleHost || !CodeHostVerifyLockWait.BelongsTo(ModuleHost) {
		t.Fatalf("owner = %q, registered = %v, want %s registered by %s", owner, registered, CodeHostVerifyLockWait, ModuleHost)
	}
}

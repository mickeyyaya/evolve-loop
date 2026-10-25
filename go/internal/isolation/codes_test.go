package isolation

import (
	"fmt"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestViolationCode_IsTheCodeOfEachInvariant(t *testing.T) {
	t.Parallel()
	if got := ViolationCode(5); got != "ISOLATION_I5_VIOLATION" {
		t.Fatalf("ViolationCode(5) = %q", got)
	}
}

func TestInit_RegistersTheNineInvariantCodesInModuleIsolation(t *testing.T) {
	t.Parallel()
	for i := 1; i <= 9; i++ {
		code := signalcenter.Code(fmt.Sprintf("ISOLATION_I%d_VIOLATION", i))
		if m, ok := signalcenter.IsRegistered(code); !ok || m != signalcenter.ModuleIsolation {
			t.Errorf("%s registered=%v module=%q, want module isolation", code, ok, m)
		}
	}
	if _, ok := signalcenter.IsRegistered("ISOLATION_I10_VIOLATION"); ok {
		t.Error("Part I has nine invariants; I-10 belongs to module healing")
	}
	for _, c := range signalcenter.RegistryConflicts() {
		if c.Module == signalcenter.ModuleIsolation || c.OtherModule == signalcenter.ModuleIsolation {
			t.Errorf("registry conflict: %+v", c)
		}
	}
}

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

func TestInit_RegistersTheContextIndexCodesInModuleIsolation(t *testing.T) {
	t.Parallel()
	for _, code := range []signalcenter.Code{CodeContextRequiredUnopened, CodeContextBudgetOver} {
		if m, ok := signalcenter.IsRegistered(code); !ok || m != signalcenter.ModuleIsolation {
			t.Errorf("%s registered=%v module=%q, want module isolation", code, ok, m)
		}
	}
	if CodeContextRequiredUnopened != "ISOLATION_CONTEXT_REQUIRED_UNOPENED" || CodeContextBudgetOver != "ISOLATION_CONTEXT_BUDGET_OVER" {
		t.Fatal("each code carries the module prefix ISOLATION_ (cell-context-index.md §6, §8)")
	}
}

func TestContextCodes_BelongToModuleIsolation(t *testing.T) {
	t.Parallel()
	for _, code := range []signalcenter.Code{CodeContextRequiredUnopened, CodeContextBudgetOver} {
		if !code.Valid() || !code.BelongsTo(signalcenter.ModuleIsolation) {
			t.Errorf("%s must be a valid code of module isolation", code)
		}
	}
}

func TestInit_RegistersTheRequestRefusalInModuleIsolation(t *testing.T) {
	t.Parallel()
	if m, ok := signalcenter.IsRegistered(CodeRequestRefused); !ok || m != signalcenter.ModuleIsolation || CodeRequestRefused != "ISOLATION_REQUEST_REFUSED" {
		t.Fatalf("%s registered=%v module=%q, want module isolation (cli-contract.md §4.5)", CodeRequestRefused, ok, m)
	}
	if m, ok := signalcenter.IsRegistered(CodeRequestFailed); !ok || m != signalcenter.ModuleIsolation || CodeRequestFailed != "ISOLATION_REQUEST_FAILED" {
		t.Fatalf("%s registered=%v module=%q, want module isolation (llm-isolation.md §7.2)", CodeRequestFailed, ok, m)
	}
	for _, k := range []signalcenter.Kind{signalcenter.KindIsolationRequest, signalcenter.KindIsolationRequestDecision} {
		if !k.Known() {
			t.Errorf("the kind %s of cli-contract.md §4.5 must be known", k)
		}
	}
}

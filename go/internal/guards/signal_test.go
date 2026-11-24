package guards

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func emitted(t *testing.T, d Denial) signalcenter.Event {
	t.Helper()
	c := signalcenter.New()
	var got []signalcenter.Event
	c.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
	EmitDenial(c, d)
	if len(got) != 1 {
		t.Fatalf("one denial emits one event, got %d", len(got))
	}
	return got[0]
}

func TestEmitDenial_OnlyTheShipGuardIsFamilyNeutral(t *testing.T) {
	t.Parallel()
	for guard, want := range map[string]string{"ship": "true", "role": "false", "phase": "false", "docdelete": "false", "unknown": "false"} {
		e := emitted(t, Denial{Guard: guard, Path: "/w/x", Reason: "no", CWD: "/w"})
		if e.Fields[core.DenialFieldNeutral] != want {
			t.Errorf("guard %s: neutral=%q, want %q", guard, e.Fields[core.DenialFieldNeutral], want)
		}
	}
}

func TestEmitDenial_TheEventCarriesTheDenialAndIsRegistered(t *testing.T) {
	t.Parallel()
	e := emitted(t, Denial{Guard: "ship", Path: "git push", Reason: "bare push", CWD: "/w/cell"})
	if e.Code != core.CodeGuardDenied || e.Module != signalcenter.ModuleIsolation || e.Fields[core.DenialFieldGuard] != "ship" ||
		e.Fields[core.DenialFieldPath] != "git push" || e.Fields[core.DenialFieldCWD] != "/w/cell" || e.Fields[core.DenialFieldReason] != "bare push" {
		t.Fatalf("event %+v", e)
	}
	if m, ok := signalcenter.IsRegistered(core.CodeGuardDenied); !ok || m != signalcenter.ModuleIsolation {
		t.Fatalf("ISOLATION_GUARD_DENIED must be registered under the isolation module: %v %v", m, ok)
	}
}

func TestFamilyNeutral_HoldsTheShipGuardAndNoOther(t *testing.T) {
	t.Parallel()
	if len(FamilyNeutral) != 1 || !FamilyNeutral["ship"] {
		t.Fatalf("only the ship hook mirrors a CLI-neutral gate: %v", FamilyNeutral)
	}
}

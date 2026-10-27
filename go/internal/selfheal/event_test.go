package selfheal

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestInit_RegistersTheHealingCodesInModuleHealing(t *testing.T) {
	t.Parallel()
	for _, code := range []signalcenter.Code{CodeIncidentOpened, CodeIncidentReopened} {
		if m, ok := signalcenter.IsRegistered(code); !ok || m != signalcenter.ModuleHealing {
			t.Errorf("%s registered=%v module=%q, want module healing", code, ok, m)
		}
	}
	for _, c := range signalcenter.RegistryConflicts() {
		if c.Module == signalcenter.ModuleHealing || c.OtherModule == signalcenter.ModuleHealing {
			t.Errorf("registry conflict: %+v", c)
		}
	}
}

func TestTransitionEvent_AnOpenIsAnIncidentKindEventWithTheRecordFields(t *testing.T) {
	t.Parallel()
	inc := Incident{ID: "inc-2026-10-27-0a1b2c3d", Fingerprint: "main-red:main", Class: ClassMainRed, Tier: TierPipeline, State: StateOpen, Severity: signalcenter.SeverityIncident, Recurrence: 2, Summary: "main is red"}
	e := Transition{Kind: TransitionOpened, Incident: inc}.Event("shadow")
	if e.Module != signalcenter.ModuleHealing || e.Kind != signalcenter.KindSelfhealIncident || e.Code != CodeIncidentOpened || e.Severity != signalcenter.SeverityIncident || e.Origin != "selfheal.Detect" {
		t.Fatalf("event %+v", e)
	}
	want := map[string]string{"incident": inc.ID, "class": "main-red", "fingerprint": "main-red:main", "tier": "T1", "state": "open", "transition": "opened", "recurrence": "2", "stage": "shadow"}
	for k, v := range want {
		if e.Fields[k] != v {
			t.Errorf("field %s = %q, want %q", k, e.Fields[k], v)
		}
	}
	if len(e.Fields) > signalcenter.MaxFields || e.Reason == "" {
		t.Fatalf("the event keeps at most %d fields and a reason: %+v", signalcenter.MaxFields, e)
	}
}

func TestTransitionEvent_AReopenIsAWarnWithItsCodeAndACloseIsAnInfoWithNoCode(t *testing.T) {
	t.Parallel()
	inc := Incident{ID: "inc-x", Severity: signalcenter.SeverityWarn}
	if e := (Transition{Kind: TransitionReopened, Incident: inc}).Event("shadow"); e.Code != CodeIncidentReopened || e.Severity != signalcenter.SeverityWarn {
		t.Fatalf("reopen %+v", e)
	}
	closedAt := t0.Add(time.Hour)
	inc.ClosedAt = &closedAt
	if e := (Transition{Kind: TransitionClosed, Incident: inc}).Event("shadow"); e.Code != "" || e.Severity != signalcenter.SeverityInfo {
		t.Fatalf("close %+v", e)
	}
}

func TestTransitionEvent_PassesTheSignalCenterNormalization(t *testing.T) {
	t.Parallel()
	c := signalcenter.New()
	var got []signalcenter.Event
	c.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
	inc := Incident{ID: "inc-x", Class: ClassQuality, Severity: signalcenter.SeverityWarn, Tier: TierPipeline, State: StateOpen}
	c.Emit(Transition{Kind: TransitionOpened, Incident: inc}.Event("shadow"))
	if len(got) != 1 || got[0].Module != signalcenter.ModuleHealing || got[0].Kind != signalcenter.KindSelfhealIncident || got[0].Code != CodeIncidentOpened {
		t.Fatalf("the center must keep the module, the kind and the code as known values: %+v", got)
	}
}

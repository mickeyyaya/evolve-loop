package orchestrator

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type specLog struct {
	mu    sync.Mutex
	specs map[string]fleet.CycleSpec
}

func (l *specLog) launch(_ context.Context, s fleet.CycleSpec) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.specs[s.Scope[0]] = s
	return 0, nil
}

func TestPool_ALaneAdmittedOnlyByTheOptimisticRuleCarriesItsWouldBeHolder(t *testing.T) {
	events, holders, launched := &eventLog{}, NewReachHolders(), &specLog{specs: map[string]fleet.CycleSpec{}}
	routerUp := make(chan struct{})
	pool := Pool{
		Target:  2,
		Admit:   AdmitByCell(admission.Admitter{Graph: importGraph(t), Optimistic: true, OnOptimistic: holders.Record}),
		Holders: holders,
		Launch: func(ctx context.Context, s fleet.CycleSpec) (int, error) {
			rc, err := launched.launch(ctx, s)
			switch s.Scope[0] {
			case "core-todo":
				<-routerUp
			case "router-todo":
				close(routerUp)
			}
			return rc, err
		},
		Publish: events.publish,
	}
	pool.Run(context.Background(), []fleet.Todo{todoIn("core-todo", "core"), todoIn("router-todo", "router")})

	if got := launched.specs["router-todo"].Env[ipcenv.FleetReachHolderKey]; got != "core-todo" {
		t.Errorf("router-todo env %s = %q, want core-todo: the reach rule would have held it behind the core lane", ipcenv.FleetReachHolderKey, got)
	}
	if got, ok := launched.specs["core-todo"].Env[ipcenv.FleetReachHolderKey]; ok {
		t.Errorf("core-todo carries a holder %q, want none: the reach rule admits it too", got)
	}
	recorded := events.withCode(CodeAdmissionOptimistic)
	want := map[string]string{"todo": "router-todo", "held_by": "core-todo", "reason": "reach meets internal/router"}
	if len(recorded) != 1 || !reflect.DeepEqual(recorded[0].Fields, want) {
		t.Fatalf("optimistic admissions = %+v, want one with %v", recorded, want)
	}
}

func TestPool_AFreshenDropKeepsNoOptimisticRecord(t *testing.T) {
	events, holders := &eventLog{}, NewReachHolders()
	holders.Record(admission.ShadowAdmission{ID: "stale", HeldBy: "core-todo", Reason: "reach meets internal/router"})
	pool := Pool{Target: 1, Holders: holders, Publish: events.publish,
		Freshen: func(fleet.CycleSpec) (fleet.CycleSpec, bool) { return fleet.CycleSpec{}, false },
		Launch:  func(context.Context, fleet.CycleSpec) (int, error) { return 0, nil },
	}
	pool.Run(context.Background(), []fleet.Todo{{ID: "stale"}})
	if got := events.withCode(CodeAdmissionOptimistic); len(got) != 0 {
		t.Errorf("optimistic admissions = %+v, want none for a todo that never launched", got)
	}
}

func TestRecordOptimisticFailure_PublishesOneSignalWithTheTodoTheHolderAndTheGate(t *testing.T) {
	events := &eventLog{}
	gateOf := func(e signalcenter.Event) (string, bool) { return e.Fields["gate"], e.Code == "SHIP_TEST_FAILED" }
	listen := RecordOptimisticFailure("router-todo", "core-todo", gateOf, events.publish)

	listen(signalcenter.Event{Code: "SHIP_TEST_OTHER"})
	listen(signalcenter.Event{Code: "SHIP_TEST_FAILED", Fields: map[string]string{"gate": "unit"}})
	listen(signalcenter.Event{Code: "SHIP_TEST_FAILED", Fields: map[string]string{"gate": "lint"}})

	got := events.withCode(CodeOptimisticAdmissionFailed)
	want := map[string]string{"todo": "router-todo", "held_by": "core-todo", "gate": "unit"}
	if len(got) != 1 || !reflect.DeepEqual(got[0].Fields, want) || got[0].Severity != signalcenter.SeverityWarn || got[0].Module != signalcenter.ModuleLoop {
		t.Fatalf("failures = %+v, want one warn loop event with %v", got, want)
	}
}

func TestOptimisticCodes_AreRegisteredToTheLoopModule(t *testing.T) {
	for code, name := range map[signalcenter.Code]string{
		CodeAdmissionOptimistic:       "LOOP_ORCH_ADMISSION_OPTIMISTIC",
		CodeOptimisticAdmissionFailed: "LOOP_ORCH_OPTIMISTIC_ADMISSION_FAILED",
	} {
		if m, ok := signalcenter.IsRegistered(code); !ok || m != signalcenter.ModuleLoop || string(code) != name {
			t.Errorf("IsRegistered(%q) = %q, %v, want %s in the loop module", code, m, ok, name)
		}
	}
}

func TestPool_AnEarlierOptimisticRecordDoesNotFollowALaterReachAdmission(t *testing.T) {
	events, holders, launched := &eventLog{}, NewReachHolders(), &specLog{specs: map[string]fleet.CycleSpec{}}
	holders.Record(admission.ShadowAdmission{ID: "router-todo", HeldBy: "core-todo", Reason: "reach meets internal/router"})
	pool := Pool{
		Target:  1,
		Admit:   AdmitByCell(admission.Admitter{Graph: importGraph(t), Optimistic: true, OnOptimistic: holders.Record}),
		Holders: holders, Launch: launched.launch, Publish: events.publish,
	}
	pool.Run(context.Background(), []fleet.Todo{todoIn("router-todo", "router")})

	if got, ok := launched.specs["router-todo"].Env[ipcenv.FleetReachHolderKey]; ok {
		t.Errorf("router-todo carries %q, want no holder: no lane was in flight at its admission", got)
	}
	if got := events.withCode(CodeAdmissionOptimistic); len(got) != 0 {
		t.Errorf("optimistic admissions = %+v, want none", got)
	}
}

func TestReachHolders_GateClearsTheLastFillAndTakeHandsEachHolderOutOnce(t *testing.T) {
	var holders *ReachHolders = NewReachHolders()
	holders.Record(admission.ShadowAdmission{ID: "stale", HeldBy: "old"})
	admit := holders.gate(func(_, _ []fleet.Todo) int {
		holders.Record(admission.ShadowAdmission{ID: "router-todo", HeldBy: "core-todo"})
		return 1
	})

	if n := admit(nil, nil); n != 1 {
		t.Fatalf("gate must return the count of the wrapped rule, got %d", n)
	}
	if _, ok := holders.take("stale"); ok {
		t.Error("a holder recorded before the fill must be cleared by the gate")
	}
	if s, ok := holders.take("router-todo"); !ok || s.HeldBy != "core-todo" {
		t.Errorf("take(router-todo) = %+v, %v; want the holder core-todo", s, ok)
	}
	if _, ok := holders.take("router-todo"); ok {
		t.Error("take must hand a holder out only once")
	}
}

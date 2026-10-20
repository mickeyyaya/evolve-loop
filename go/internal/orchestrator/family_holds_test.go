package orchestrator

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func TestFamilyHolds_TheUnderfilledWarnNamesEachFamilyHold(t *testing.T) {
	t.Parallel()
	holds := NewFamilyHolds()
	holds.Record(admission.Decision{ID: "ctx-telemetry", Cells: []string{"family:ctx", "internal/x"}, BlockedBy: "ctx-cap"})
	holds.Record(admission.Decision{ID: "pkg-held", Cells: []string{"internal/y"}, BlockedBy: "lane-2"})
	events := &eventLog{}

	Pool{MinLanes: 4, Publish: events.publish, FamilyHolds: holds}.transition(fleet.PoolTransition{Kind: fleet.PoolIdle, Live: 1, Target: 4, Pending: 2})

	if got := fieldsOf(events, CodePoolUnderfilled, "family_held"); len(got) != 1 || got[0] != "ctx-telemetry (family:ctx, behind ctx-cap)" {
		t.Fatalf("family_held = %q, want the one family hold, so the WARN does not read as a supply failure", got)
	}
}

func TestFamilyHolds_ALaunchedTodoLeavesTheList(t *testing.T) {
	t.Parallel()
	for _, kind := range []fleet.PoolTransitionKind{fleet.PoolLaunched, fleet.PoolBackfilled} {
		holds := NewFamilyHolds()
		holds.Record(admission.Decision{ID: "ctx-telemetry", Cells: []string{"family:ctx"}, BlockedBy: "ctx-cap"})
		events := &eventLog{}
		p := Pool{MinLanes: 4, Publish: events.publish, FamilyHolds: holds}

		p.transition(fleet.PoolTransition{Kind: kind, Todo: "ctx-telemetry", Live: 1, Target: 4})
		p.transition(fleet.PoolTransition{Kind: fleet.PoolIdle, Live: 1, Target: 4})

		warns := events.withCode(CodePoolUnderfilled)
		if len(warns) != 1 {
			t.Fatalf("%s: underfilled events = %d, want 1", kind, len(warns))
		}
		if _, ok := warns[0].Fields["family_held"]; ok {
			t.Errorf("%s: fields = %v: a launched todo is no longer held", kind, warns[0].Fields)
		}
	}
}

func TestFamilyHolds_AHoldThatLosesItsFamilyCellLeavesTheList(t *testing.T) {
	t.Parallel()
	holds := NewFamilyHolds()
	holds.Record(admission.Decision{ID: "ctx-telemetry", Cells: []string{"family:ctx"}, BlockedBy: "ctx-cap"})
	holds.Record(admission.Decision{ID: "ctx-telemetry", Cells: []string{"internal/x"}, BlockedBy: "lane-2"})
	if got := holds.Summary(); got != "" {
		t.Fatalf("summary = %q: the todo now waits on a package cell, so the family hold is gone", got)
	}
	var none *FamilyHolds
	none.Record(admission.Decision{ID: "x", Cells: []string{"family:x"}})
	if none.Summary() != "" {
		t.Error("a nil hold list records nothing")
	}
}

func TestPool_TwoTodosOfOneFamilyRunOneAfterTheOther(t *testing.T) {
	t.Parallel()
	g, events, holds := newGatedLaunch(), &eventLog{}, NewFamilyHolds()
	admit := AdmitByCell(admission.Admitter{Graph: importGraph(t), OnWait: holds.Record})
	backlog := []fleet.Todo{{ID: "ctx-cap", Files: []string{"docs/a.md"}}, {ID: "solo", Files: []string{"docs/c.md"}}, {ID: "ctx-telemetry", Files: []string{"docs/b.md"}}}
	done := runPool(Pool{Target: 4, MinLanes: 4, Admit: admit, Launch: g.launch, Publish: events.publish, FamilyHolds: holds}, backlog)

	first, second := waitStart(t, g), waitStart(t, g)
	if first != "ctx-cap" || second != "solo" {
		t.Fatalf("launched %q then %q, want ctx-cap and solo: ctx-telemetry waits for its sibling", first, second)
	}
	g.finish("solo", 0)
	select {
	case id := <-g.started:
		t.Fatalf("%s launched beside its sibling: one family runs one lane at a time", id)
	case <-time.After(200 * time.Millisecond):
	}
	if got := fieldsOf(events, CodePoolUnderfilled, "family_held"); len(got) == 0 || got[len(got)-1] != "ctx-telemetry (family:ctx, behind ctx-cap)" {
		t.Errorf("family_held = %q, want the sibling named while it waits", got)
	}
	g.finish("ctx-cap", 0)
	if got := waitStart(t, g); got != "ctx-telemetry" {
		t.Fatalf("launched %q, want ctx-telemetry once its sibling exits", got)
	}
	g.finish("ctx-telemetry", 0)
	<-done
}

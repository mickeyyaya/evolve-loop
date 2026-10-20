package admission_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func TestAdmit_TwoTodosOfOneIDFamilyWaitForEachOther(t *testing.T) {
	for _, optimistic := range []bool{false, true} {
		t.Run(fmt.Sprintf("optimistic=%v", optimistic), func(t *testing.T) {
			admitTwoSiblings(t, optimistic)
		})
	}
}

func admitTwoSiblings(t *testing.T, optimistic bool) {
	t.Helper()
	a := admitter(t, nil)
	a.Optimistic = optimistic
	candidates := []fleet.Todo{
		todo("context-fill-cap", "go/internal/quotastate/windows.go"),
		todo("context-fill-telemetry", "go/internal/inboxrank/inboxrank.go"),
	}
	admitted, waiting := a.Admit(nil, candidates, 32)
	if got := ids(admitted); !reflect.DeepEqual(got, []string{"context-fill-cap"}) {
		t.Fatalf("admitted = %v, want only context-fill-cap: its sibling shares the family", got)
	}
	want := admission.Decision{ID: "context-fill-telemetry", Rank: 1, Cells: []string{"family:context", "internal/inboxrank"}, BlockedBy: "context-fill-cap"}
	if len(waiting) != 1 || !reflect.DeepEqual(waiting[0], want) {
		t.Fatalf("waiting = %+v, want [%+v]", waiting, want)
	}
}

func TestAdmit_ALaneInFlightHoldsItsIDFamily(t *testing.T) {
	a := admitter(t, nil)
	held := []admission.Holding{{Lane: "lane-1", Todo: todo("tokenopt-cache-discipline-audit", "go/internal/quotastate/windows.go")}}
	admitted, waiting := a.Admit(held, []fleet.Todo{todo("tokenopt-session-resume-on-retry", "go/internal/inboxrank/inboxrank.go")}, 32)
	if len(admitted) != 0 || len(waiting) != 1 || waiting[0].BlockedBy != "lane-1" {
		t.Fatalf("admitted = %v, waiting = %+v: the tokenopt lane in flight holds the tokenopt family", ids(admitted), waiting)
	}
}

func TestAdmit_TodosOfDifferentIDFamiliesAreUnaffected(t *testing.T) {
	a := admitter(t, nil)
	candidates := []fleet.Todo{
		todo("tokenopt-cache-discipline-audit", "go/internal/quotastate/windows.go"),
		todo("context-fill-telemetry", "go/internal/inboxrank/inboxrank.go"),
		todo("solo", "docs/a.md"),
		todo("-lone", "docs/b.md"),
		todo("-stray", "docs/c.md"),
	}
	admitted, waiting := a.Admit(nil, candidates, 32)
	if got := ids(admitted); len(waiting) != 0 || !reflect.DeepEqual(got, []string{"tokenopt-cache-discipline-audit", "context-fill-telemetry", "solo", "-lone", "-stray"}) {
		t.Fatalf("admitted = %v, waiting = %+v: disjoint families and ids without a family run together", got, waiting)
	}
	for _, d := range admitted[2:] {
		if len(d.Cells) != 0 {
			t.Errorf("%s has no family token, cells = %v, want none", d.ID, d.Cells)
		}
	}
}

func TestAdmit_AFamilyHoldIsNoReachShadow(t *testing.T) {
	for _, optimistic := range []bool{false, true} {
		calls := 0
		a := admitter(t, nil)
		a.Optimistic = optimistic
		a.OnShadow = func(admission.ShadowAdmission) { calls++ }
		a.OnOptimistic = func(admission.ShadowAdmission) { calls++ }
		held := []admission.Holding{{Lane: "lane-1", Todo: todo("context-fill-cap", "go/internal/quotastate/windows.go")}}
		_, waiting := a.Admit(held, []fleet.Todo{todo("context-fill-telemetry", "go/internal/inboxrank/inboxrank.go")}, 32)
		if len(waiting) != 1 || calls != 0 {
			t.Fatalf("optimistic=%v: waiting = %+v, sink calls = %d, want the family hold with no reach record", optimistic, waiting, calls)
		}
	}
}

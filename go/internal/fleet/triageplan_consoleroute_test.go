package fleet

// triageplan_consoleroute_test.go — RED contract for ADR-0074 I1 at the REAL
// dispatch chokepoint (architect review finding 2): triage-decision.json's
// top_n is the load-bearing selection→build handoff, consumed here by BOTH
// schedulers (wave PlanFromTriage, pool TodosFromTriage) and by the
// wave-seed-inbox fallback. A console-routed id must be refused at plan time
// — loudly (refusals list) — never silently planned into a lane.

import (
	"strings"
	"testing"
)

func stubRouted(routedIDs map[string]string) RoutedFn {
	return func(id string) (bool, string) {
		reason, ok := routedIDs[id]
		return ok, reason
	}
}

func TestTodosFromTriage_RefusesConsoleRoutedIds(t *testing.T) {
	decision := []byte(`{"top_n":[{"id":"lane-work"},{"id":"operator-work"}]}`)
	todos, refused, err := TodosFromTriage(decision, nil, stubRouted(map[string]string{"operator-work": "route:console-manual"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 1 || todos[0].ID != "lane-work" {
		t.Fatalf("todos = %+v, want only lane-work", todos)
	}
	if len(refused) != 1 || !strings.Contains(refused[0], "operator-work") || !strings.Contains(refused[0], "console-manual") {
		t.Fatalf("refusals must name the id and reason, got %v", refused)
	}
}

// committed_floors ids pass through the same gate — the precedence source must
// not be a bypass.
func TestTodosFromTriage_RefusesRoutedCommittedFloor(t *testing.T) {
	decision := []byte(`{"committed_floors":["operator-work","lane-work"]}`)
	todos, refused, err := TodosFromTriage(decision, nil, stubRouted(map[string]string{"operator-work": "protected fix surface: go/internal/guards/role.go"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 1 || todos[0].ID != "lane-work" {
		t.Fatalf("todos = %+v, want only lane-work", todos)
	}
	if len(refused) != 1 {
		t.Fatalf("want 1 refusal, got %v", refused)
	}
}

// nil resolver = no routing context (unit tests, legacy callers) — everything
// dispatchable, zero refusals; unknown ids under a real resolver behave the
// same (scout-originated work has no inbox item and must not be blocked).
func TestTodosFromTriage_NilResolverKeepsAll(t *testing.T) {
	decision := []byte(`{"top_n":[{"id":"a"},{"id":"b"}]}`)
	todos, refused, err := TodosFromTriage(decision, nil, nil)
	if err != nil || len(todos) != 2 || len(refused) != 0 {
		t.Fatalf("nil resolver must keep all: todos=%d refused=%v err=%v", len(todos), refused, err)
	}
}

func TestPlanFromTriage_PropagatesRefusals(t *testing.T) {
	decision := []byte(`{"top_n":[{"id":"lane-work"},{"id":"operator-work"}]}`)
	specs, refused, err := PlanFromTriage(decision, nil, 2, stubRouted(map[string]string{"operator-work": "route:console-salvage"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(refused) != 1 {
		t.Fatalf("refusals must propagate through PlanFromTriage, got %v", refused)
	}
	for _, s := range specs {
		if strings.Contains(strings.Join(s.Scope, " "), "operator-work") {
			t.Fatalf("refused id leaked into a lane spec: %+v", s)
		}
	}
}

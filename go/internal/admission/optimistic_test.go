package admission_test

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func optimisticAdmitter(t *testing.T) admission.Admitter {
	t.Helper()
	a := admitter(t, nil)
	a.Optimistic = true
	return a
}

func shadowAdmitter(t *testing.T) (admission.Admitter, *[]admission.ShadowAdmission) {
	t.Helper()
	records := &[]admission.ShadowAdmission{}
	a := admitter(t, nil)
	a.OnShadow = func(s admission.ShadowAdmission) { *records = append(*records, s) }
	return a, records
}

func TestAdmit_OptimisticAdmitsACandidateThatOnlyTheRunningLaneImports(t *testing.T) {
	cases := []struct {
		name    string
		running admission.Holding
		cand    fleet.Todo
	}{
		{"a fleet lane imports the inboxrank candidate", admission.Holding{Lane: "lane-1", Todo: todo("f", "go/internal/fleet/pool.go")}, todo("i", "go/internal/inboxrank/inboxrank.go")},
		{"an inboxrank lane is imported by the fleet candidate", admission.Holding{Lane: "lane-1", Todo: todo("i", "go/internal/inboxrank/inboxrank.go")}, todo("f", "go/internal/fleet/pool.go")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, waiting := admitter(t, nil).Admit([]admission.Holding{tc.running}, []fleet.Todo{tc.cand}, 32); len(waiting) != 1 {
				t.Fatalf("precondition: the reach rule must hold %s, waiting = %+v", tc.cand.ID, waiting)
			}
			admitted, waiting := optimisticAdmitter(t).Admit([]admission.Holding{tc.running}, []fleet.Todo{tc.cand}, 32)
			if got := ids(admitted); !reflect.DeepEqual(got, []string{tc.cand.ID}) || len(waiting) != 0 {
				t.Fatalf("admitted = %v, waiting = %+v, want %s admitted: the other lane only imports its package", got, waiting, tc.cand.ID)
			}
		})
	}
}

func TestAdmit_OptimisticStillHoldsTwoTodosThatEditOnePackage(t *testing.T) {
	a := optimisticAdmitter(t)
	held := []admission.Holding{{Lane: "lane-1", Todo: todo("a", "go/internal/inboxrank/inboxrank.go")}}
	admitted, waiting := a.Admit(held, []fleet.Todo{todo("d", "go/internal/inboxrank/explain.go"), todo("q", "go/internal/quotastate/windows.go")}, 32)
	if got := ids(admitted); !reflect.DeepEqual(got, []string{"q"}) {
		t.Fatalf("admitted = %v, want [q]", got)
	}
	if len(waiting) != 1 || waiting[0].ID != "d" || waiting[0].BlockedBy != "lane-1" {
		t.Fatalf("waiting = %+v, want d held by lane-1: both edit inboxrank", waiting)
	}
	admitted, waiting = a.Admit(nil, []fleet.Todo{todo("c1", "go/internal/fleet/a.go"), todo("c2", "go/internal/fleet/b.go"), todo("c3", "go/internal/fleet/c.go")}, 32)
	if got := ids(admitted); !reflect.DeepEqual(got, []string{"c1"}) {
		t.Fatalf("admitted = %v, want [c1]: three candidates of one package never run at once", got)
	}
	if got := ids(waiting); !reflect.DeepEqual(got, []string{"c2", "c3"}) || waiting[0].BlockedBy != "c1" || waiting[1].BlockedBy != "c1" {
		t.Fatalf("waiting = %+v, want c2 and c3 held by c1", waiting)
	}
}

func TestAdmit_OptimisticKeepsTheHubRule(t *testing.T) {
	cases := []struct {
		name    string
		running fleet.Todo
		cand    fleet.Todo
	}{
		{"two edits of the hub package", todo("h1", "go/internal/sysexec/command.go"), todo("h2", "go/internal/sysexec/exec.go")},
		{"a hub package and go.mod", todo("h1", "go/internal/sysexec/command.go"), todo("h2", "go/go.mod")},
		{"go.mod and the policy file", todo("h1", "go/go.mod"), todo("h2", ".evolve/policy.json")},
		{"a package and a todo with no files", todo("h1", "go/internal/sysexec/command.go"), todo("h2")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			admitted, waiting := optimisticAdmitter(t).Admit([]admission.Holding{{Lane: "lane-1", Todo: tc.running}}, []fleet.Todo{tc.cand}, 32)
			if len(admitted) != 0 || len(waiting) != 1 || waiting[0].BlockedBy != "lane-1" || waiting[0].Cells[0] != admission.HubCell {
				t.Fatalf("admitted = %+v, waiting = %+v, want %s held in the hub cell by lane-1", admitted, waiting, tc.cand.ID)
			}
		})
	}
	admitted, _ := optimisticAdmitter(t).Admit([]admission.Holding{{Lane: "lane-1", Todo: todo("h", "go/internal/sysexec/command.go")}}, []fleet.Todo{todo("q", "go/internal/quotastate/windows.go")}, 32)
	if got := ids(admitted); !reflect.DeepEqual(got, []string{"q"}) {
		t.Errorf("admitted = %v, want [q]: one hub holder does not hold a non-hub todo", got)
	}
}

func TestAdmit_OptimisticReportsTheSameCellsAndRanksAsTheReachRule(t *testing.T) {
	candidates := []fleet.Todo{todo("q", "go/internal/quotastate/windows.go"), todo("h", "go/go.mod"), todo("docs", "docs/a.md")}
	want, wantWait := admitter(t, nil).Admit(nil, candidates, 32)
	got, gotWait := optimisticAdmitter(t).Admit(nil, candidates, 32)
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gotWait, wantWait) {
		t.Fatalf("optimistic = %+v / %+v, reach rule = %+v / %+v, want the same decisions where no import is involved", got, gotWait, want, wantWait)
	}
}

func TestAdmit_ShadowRecordsEachAdmissionTheReachRuleHoldsWithItsHolder(t *testing.T) {
	a, records := shadowAdmitter(t)
	held := []admission.Holding{{Lane: "lane-1", Todo: todo("f", "go/internal/fleet/pool.go")}}
	candidates := []fleet.Todo{
		todo("i", "go/internal/inboxrank/inboxrank.go"),
		todo("q", "go/internal/quotastate/windows.go"),
		todo("f2", "go/internal/fleet/other.go"),
	}
	admitted, waiting := a.Admit(held, candidates, 32)
	if got := ids(admitted); !reflect.DeepEqual(got, []string{"q"}) {
		t.Fatalf("admitted = %v, want [q]: the shadow leaves the reach rule in force", got)
	}
	if got := ids(waiting); !reflect.DeepEqual(got, []string{"i", "f2"}) {
		t.Fatalf("waiting = %v, want [i f2]", got)
	}
	want := []admission.ShadowAdmission{{ID: "i", Rank: 0, Cells: []string{"internal/inboxrank"}, HeldBy: "lane-1"}}
	if !reflect.DeepEqual(*records, want) {
		t.Fatalf("records = %+v, want %+v: only i is held by import alone; f2 shares fleet with lane-1", *records, want)
	}
}

func TestAdmit_ShadowNamesTheCandidateThatTookTheCellAsTheHolder(t *testing.T) {
	a, records := shadowAdmitter(t)
	a.Admit(nil, []fleet.Todo{todo("f", "go/internal/fleet/pool.go"), todo("i", "go/internal/inboxrank/inboxrank.go")}, 32)
	want := []admission.ShadowAdmission{{ID: "i", Rank: 1, Cells: []string{"internal/inboxrank"}, HeldBy: "f"}}
	if !reflect.DeepEqual(*records, want) {
		t.Fatalf("records = %+v, want %+v: f was admitted first and its import holds i", *records, want)
	}
}

func TestAdmit_ShadowRecordsNothingForAHubOrSharedEditHold(t *testing.T) {
	a, records := shadowAdmitter(t)
	held := []admission.Holding{
		{Lane: "lane-1", Todo: todo("h", "go/internal/sysexec/command.go")},
		{Lane: "lane-2", Todo: todo("q", "go/internal/quotastate/windows.go")},
	}
	_, waiting := a.Admit(held, []fleet.Todo{todo("h2", "go/go.mod"), todo("q2", "go/internal/quotastate/state.go")}, 32)
	if len(waiting) != 2 {
		t.Fatalf("waiting = %+v, want both held", waiting)
	}
	if len(*records) != 0 {
		t.Fatalf("records = %+v, want none: the new rule holds both too", *records)
	}
}

func TestAdmit_OptimisticNeverCallsTheShadowSink(t *testing.T) {
	a, records := shadowAdmitter(t)
	a.Optimistic = true
	held := []admission.Holding{{Lane: "lane-1", Todo: todo("f", "go/internal/fleet/pool.go")}}
	admitted, _ := a.Admit(held, []fleet.Todo{todo("i", "go/internal/inboxrank/inboxrank.go")}, 32)
	if got := ids(admitted); !reflect.DeepEqual(got, []string{"i"}) || len(*records) != 0 {
		t.Fatalf("admitted = %v, records = %+v, want i admitted and no shadow record once the new rule governs", got, *records)
	}
}

func TestAdmit_TheShadowChangesNoDecisionOverRandomBacklogs(t *testing.T) {
	plain := admitter(t, nil)
	sunk := admitter(t, nil)
	recorded := 0
	sunk.OnShadow = func(admission.ShadowAdmission) { recorded++ }
	r := rand.New(rand.NewSource(20261019))
	for round := range 500 {
		var held []admission.Holding
		for i := range r.Intn(3) {
			held = append(held, admission.Holding{Lane: "h" + string(rune('a'+i)), Todo: randomTodo(r, "h"+string(rune('a'+i)))})
		}
		var candidates []fleet.Todo
		for i := range 1 + r.Intn(8) {
			candidates = append(candidates, randomTodo(r, "c"+string(rune('a'+i))))
		}
		scanK := 1 + r.Intn(6)
		wantAdmitted, wantWaiting := plain.Admit(held, candidates, scanK)
		gotAdmitted, gotWaiting := sunk.Admit(held, candidates, scanK)
		if !reflect.DeepEqual(gotAdmitted, wantAdmitted) || !reflect.DeepEqual(gotWaiting, wantWaiting) {
			t.Fatalf("round %d: with a shadow sink admitted = %+v waiting = %+v, want %+v / %+v", round, gotAdmitted, gotWaiting, wantAdmitted, wantWaiting)
		}
	}
	if recorded == 0 {
		t.Fatal("the random backlogs never exercised a shadow record: the property proves nothing")
	}
}

func TestAdmit_OptimisticAdmitsExactlyTheTodosThatShareNoPackageAndAreNotBothHubs(t *testing.T) {
	a := optimisticAdmitter(t)
	r := rand.New(rand.NewSource(20261019))
	for round := range 500 {
		files := map[string][]string{}
		var held []admission.Holding
		for i := range r.Intn(3) {
			td := randomTodo(r, "h"+string(rune('a'+i)))
			files[td.ID] = td.Files
			if !sharesAPackageOrBothHubs(held, td.Files, files) {
				held = append(held, admission.Holding{Lane: td.ID, Todo: td})
			}
		}
		var candidates []fleet.Todo
		for i := range 1 + r.Intn(8) {
			td := randomTodo(r, "c"+string(rune('a'+i)))
			files[td.ID] = td.Files
			candidates = append(candidates, td)
		}
		admitted, waiting := a.Admit(held, candidates, 1+r.Intn(6))
		running := make([]string, 0, len(held)+len(admitted))
		for _, h := range held {
			running = append(running, h.Lane)
		}
		for _, d := range admitted {
			for _, other := range running {
				if oracleSharedEdit(files[d.ID], files[other]) {
					t.Fatalf("round %d: admitted %s %v shares a package or the hub with %s %v", round, d.ID, files[d.ID], other, files[other])
				}
			}
			running = append(running, d.ID)
		}
		for _, w := range waiting {
			if !oracleSharedEdit(files[w.ID], files[w.BlockedBy]) {
				t.Fatalf("round %d: %s %v waits on %s %v, which shares no package and is not a second hub", round, w.ID, files[w.ID], w.BlockedBy, files[w.BlockedBy])
			}
		}
	}
}

func oracleSharedEdit(a, b []string) bool {
	ea, ha := oracleFootprint(a)
	eb, hb := oracleFootprint(b)
	if ha && hb {
		return true
	}
	for p := range ea {
		if eb[p] {
			return true
		}
	}
	return false
}

func sharesAPackageOrBothHubs(held []admission.Holding, f []string, files map[string][]string) bool {
	for _, h := range held {
		if oracleSharedEdit(f, files[h.Lane]) {
			return true
		}
	}
	return false
}

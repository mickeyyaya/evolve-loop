package admission_test

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

var coreModule = map[string][]string{
	"internal/router":    nil,
	"internal/clihealth": nil,
	"internal/core":      {"internal/router", "internal/clihealth"},
	"cmd/evolve":         {"internal/core"},
}

func coreAdmitter(t *testing.T, onlySharedEdits bool, shadow func(admission.Shadowed)) admission.Admitter {
	t.Helper()
	g, err := admission.NewModuleGraph(listing(coreModule), 10)
	if err != nil {
		t.Fatal(err)
	}
	return admission.Admitter{Graph: g, OnlySharedEdits: onlySharedEdits, Shadow: shadow}
}

func coreLane() []admission.Holding {
	return []admission.Holding{{Lane: "lane-core", Todo: todo("c", "go/internal/core/task_contract.go")}}
}

func recorder() (*[]admission.Shadowed, func(admission.Shadowed)) {
	var got []admission.Shadowed
	return &got, func(s admission.Shadowed) { got = append(got, s) }
}

func TestAdmit_TheSharedEditRuleAdmitsAnImportOfARunningCoreLane(t *testing.T) {
	r := todo("r", "go/internal/router/router.go")
	admitted, waiting := coreAdmitter(t, false, nil).Admit(coreLane(), []fleet.Todo{r}, 32)
	if len(admitted) != 0 || len(waiting) != 1 || waiting[0].BlockedBy != "lane-core" {
		t.Fatalf("reach rule: admitted = %v, waiting = %+v, want r held by lane-core: core imports router", ids(admitted), waiting)
	}
	admitted, waiting = coreAdmitter(t, true, nil).Admit(coreLane(), []fleet.Todo{r}, 32)
	if got := ids(admitted); len(waiting) != 0 || !reflect.DeepEqual(got, []string{"r"}) {
		t.Fatalf("shared-edit rule: admitted = %v, waiting = %+v, want r admitted: r and lane-core edit no common package", got, waiting)
	}
}

func TestAdmit_UnderTheSharedEditRuleTwoEditsOfOnePackageNeverRunAtOnce(t *testing.T) {
	a := coreAdmitter(t, true, nil)
	admitted, waiting := a.Admit(coreLane(), []fleet.Todo{todo("r1", "go/internal/router/a.go"), todo("r2", "go/internal/router/b.go"), todo("c2", "go/internal/core/x.go")}, 32)
	if got := ids(admitted); !reflect.DeepEqual(got, []string{"r1"}) {
		t.Fatalf("admitted = %v, want [r1]", got)
	}
	want := []admission.Decision{
		{ID: "r2", Rank: 1, Cells: []string{"internal/router"}, BlockedBy: "r1"},
		{ID: "c2", Rank: 2, Cells: []string{"internal/core"}, BlockedBy: "lane-core"},
	}
	if !reflect.DeepEqual(waiting, want) {
		t.Fatalf("waiting = %+v, want %+v: a shared package still holds", waiting, want)
	}
}

func TestAdmit_UnderTheSharedEditRuleTheHubRuleIsUnchanged(t *testing.T) {
	a := admission.Admitter{Graph: admissionGraph(t), OnlySharedEdits: true}
	held := []admission.Holding{{Lane: "lane-1", Todo: todo("e", "go/internal/sysexec/command.go")}}
	admitted, waiting := a.Admit(held, []fleet.Todo{todo("m", "go/go.mod"), todo("q", "go/internal/quotastate/windows.go"), todo("n")}, 32)
	if got := ids(admitted); !reflect.DeepEqual(got, []string{"q"}) {
		t.Fatalf("admitted = %v, want [q]: an import of the hub never conflicts", got)
	}
	if got := ids(waiting); !reflect.DeepEqual(got, []string{"m", "n"}) || waiting[0].BlockedBy != "lane-1" || waiting[1].BlockedBy != "lane-1" {
		t.Fatalf("waiting = %+v, want m and n held by lane-1: two hubs never run at once", waiting)
	}
}

func TestAdmit_AtShadowTheAdmittedSetIsTodaysAndTheShadowNamesTheHolder(t *testing.T) {
	candidates := []fleet.Todo{todo("r", "go/internal/router/router.go"), todo("c2", "go/internal/core/x.go"), todo("h", "go/internal/clihealth/h.go")}
	todayAdmitted, todayWaiting := coreAdmitter(t, false, nil).Admit(coreLane(), candidates, 32)
	got, record := recorder()
	admitted, waiting := coreAdmitter(t, false, record).Admit(coreLane(), candidates, 32)
	if !reflect.DeepEqual(admitted, todayAdmitted) || !reflect.DeepEqual(waiting, todayWaiting) {
		t.Fatalf("shadow admitted %+v waiting %+v, want today's %+v %+v", admitted, waiting, todayAdmitted, todayWaiting)
	}
	want := []admission.Shadowed{
		{Todo: "r", Holder: "lane-core", Reason: "reach meets internal/router"},
		{Todo: "h", Holder: "lane-core", Reason: "reach meets internal/clihealth"},
	}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("shadow = %+v, want %+v: c2 shares core, so the new rule holds it too", *got, want)
	}
}

func TestAdmit_TheShadowNamesTheReachOfTheCandidate(t *testing.T) {
	got, record := recorder()
	held := []admission.Holding{{Lane: "lane-router", Todo: todo("r", "go/internal/router/router.go")}}
	_, waiting := coreAdmitter(t, false, record).Admit(held, []fleet.Todo{todo("c", "go/internal/core/x.go")}, 32)
	want := []admission.Shadowed{{Todo: "c", Holder: "lane-router", Reason: "reach meets internal/router"}}
	if len(waiting) != 1 || !reflect.DeepEqual(*got, want) {
		t.Fatalf("waiting = %+v, shadow = %+v, want c held and recorded as %+v", waiting, *got, want)
	}
}

func TestAdmit_AtEnforceTheSharedEditRuleDecidesAndRecordsNothing(t *testing.T) {
	got, record := recorder()
	admitted, waiting := coreAdmitter(t, true, record).Admit(coreLane(), []fleet.Todo{todo("r", "go/internal/router/router.go"), todo("c2", "go/internal/core/x.go")}, 32)
	if a, w := ids(admitted), ids(waiting); !reflect.DeepEqual(a, []string{"r"}) || !reflect.DeepEqual(w, []string{"c2"}) {
		t.Fatalf("admitted = %v, waiting = %v, want r admitted and c2 held", a, w)
	}
	if len(*got) != 0 {
		t.Errorf("shadow = %+v, want none at enforce", *got)
	}
}

func oracleSharedConflict(a, b []string) bool {
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

func TestAdmit_UnderTheSharedEditRuleTwoAdmittedTodosNeverShareAnEdit(t *testing.T) {
	a := admission.Admitter{Graph: admissionGraph(t), OnlySharedEdits: true}
	r := rand.New(rand.NewSource(20261019))
	for round := range 500 {
		files := map[string][]string{}
		var candidates []fleet.Todo
		for i := range 1 + r.Intn(8) {
			td := randomTodo(r, "c"+string(rune('a'+i)))
			files[td.ID] = td.Files
			candidates = append(candidates, td)
		}
		admitted, waiting := a.Admit(nil, candidates, 32)
		for i, d := range admitted {
			for _, other := range admitted[:i] {
				if oracleSharedConflict(files[d.ID], files[other.ID]) {
					t.Fatalf("round %d: admitted %s %v shares an edit or the hub with %s %v", round, d.ID, files[d.ID], other.ID, files[other.ID])
				}
			}
		}
		for _, w := range waiting {
			if !oracleSharedConflict(files[w.ID], files[w.BlockedBy]) {
				t.Fatalf("round %d: %s %v waits on %s %v, which shares no edit", round, w.ID, files[w.ID], w.BlockedBy, files[w.BlockedBy])
			}
		}
	}
}

func TestAdmit_TheShadowSkipsATodoThatALaterHolderHoldsByASharedEdit(t *testing.T) {
	got, record := recorder()
	held := append(coreLane(), admission.Holding{Lane: "lane-router", Todo: todo("r0", "go/internal/router/a.go")})
	_, waiting := coreAdmitter(t, false, record).Admit(held, []fleet.Todo{todo("r", "go/internal/router/b.go")}, 32)
	if len(waiting) != 1 || waiting[0].BlockedBy != "lane-core" {
		t.Fatalf("waiting = %+v, want r held by lane-core first", waiting)
	}
	if len(*got) != 0 {
		t.Errorf("shadow = %+v, want none: lane-router edits router too, so the shared-edit rule also holds r", *got)
	}
}

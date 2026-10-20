//go:build acs

package cycle1995

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

var fixtureModule = map[string][]string{
	"internal/sysexec": nil,
	"internal/ipcenv":  {"internal/sysexec"},
	"internal/router":  {"internal/sysexec", "internal/ipcenv"},
	"internal/core":    {"internal/sysexec", "internal/router"},
	"internal/fleet":   {"internal/sysexec", "internal/core"},
	"cmd/evolve":       {"internal/core", "internal/router", "internal/fleet"},
}

func fixtureGraph(t *testing.T) *admission.ModuleGraph {
	t.Helper()
	var b strings.Builder
	for pkg, deps := range fixtureModule {
		full := []string{"fmt"}
		for _, d := range deps {
			full = append(full, admission.ModulePath+"/"+d)
		}
		b.WriteString(admission.ModulePath + "/" + pkg + "|" + strings.Join(full, " ") + "\n")
	}
	g, err := admission.NewModuleGraph(b.String(), 4)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func todo(id string, files ...string) fleet.Todo { return fleet.Todo{ID: id, Files: files} }

func ids(ds []admission.Decision) []string {
	out := []string{}
	for _, d := range ds {
		out = append(out, d.ID)
	}
	return out
}

func TestC1995_001_ALaneEditingCoreDoesNotHoldACandidateEditingRouter(t *testing.T) {
	running := []admission.Holding{{Lane: "core-lane", Todo: todo("core-lane", "go/internal/core/state.go")}}
	candidate := todo("router-todo", "go/internal/router/router.go")

	_, heldToday := admission.Admitter{Graph: fixtureGraph(t)}.Admit(running, []fleet.Todo{candidate}, 32)
	if len(heldToday) != 1 || heldToday[0].BlockedBy != "core-lane" {
		t.Fatalf("RED precondition: the reach rule must hold router-todo behind core-lane, waiting = %+v", heldToday)
	}

	admitted, waiting := admission.Admitter{Graph: fixtureGraph(t), Optimistic: true}.Admit(running, []fleet.Todo{candidate}, 32)
	if !reflect.DeepEqual(ids(admitted), []string{"router-todo"}) || len(waiting) != 0 {
		t.Errorf("RED: admitted = %v, waiting = %+v, want router-todo admitted beside core-lane", ids(admitted), waiting)
	}
}

func TestC1995_002_TwoCandidatesOfOnePackageNeverRunTogetherAndTheHubRuleHolds(t *testing.T) {
	a := admission.Admitter{Graph: fixtureGraph(t), Optimistic: true}

	admitted, waiting := a.Admit(nil, []fleet.Todo{
		todo("core-one", "go/internal/core/a.go"),
		todo("core-two", "go/internal/core/b.go"),
		todo("router-one", "go/internal/router/a.go"),
	}, 32)
	if !reflect.DeepEqual(ids(admitted), []string{"core-one", "router-one"}) {
		t.Errorf("RED: admitted = %v, want core-one and router-one: core-two shares the core package", ids(admitted))
	}
	if len(waiting) != 1 || waiting[0].ID != "core-two" || waiting[0].BlockedBy != "core-one" {
		t.Errorf("RED: waiting = %+v, want core-two held by core-one", waiting)
	}

	admitted, waiting = a.Admit(
		[]admission.Holding{{Lane: "hub-lane", Todo: todo("hub-lane", "go/go.mod")}},
		[]fleet.Todo{todo("sysexec-todo", "go/internal/sysexec/command.go"), todo("router-todo", "go/internal/router/a.go")}, 32)
	if !reflect.DeepEqual(ids(admitted), []string{"router-todo"}) || len(waiting) != 1 || waiting[0].ID != "sysexec-todo" || waiting[0].Cells[0] != admission.HubCell {
		t.Errorf("RED: admitted = %v, waiting = %+v, want the second hub todo held and the router todo admitted", ids(admitted), waiting)
	}
}

func TestC1995_003_AtShadowTheAdmittedSetIsTodaysAndEachNewAdmissionNamesItsHolder(t *testing.T) {
	running := []admission.Holding{{Lane: "core-lane", Todo: todo("core-lane", "go/internal/core/state.go")}}
	candidates := []fleet.Todo{
		todo("router-todo", "go/internal/router/router.go"),
		todo("ipcenv-todo", "go/internal/ipcenv/env.go"),
		todo("core-two", "go/internal/core/other.go"),
		todo("docs-todo", "docs/a.md"),
	}
	wantAdmitted, wantWaiting := admission.Admitter{Graph: fixtureGraph(t)}.Admit(running, candidates, 32)

	var records []admission.ShadowAdmission
	shadow := admission.Admitter{Graph: fixtureGraph(t), OnShadow: func(s admission.ShadowAdmission) { records = append(records, s) }}
	gotAdmitted, gotWaiting := shadow.Admit(running, candidates, 32)

	if !reflect.DeepEqual(gotAdmitted, wantAdmitted) || !reflect.DeepEqual(gotWaiting, wantWaiting) {
		t.Errorf("RED: with a shadow sink admitted = %+v waiting = %+v, want today's %+v / %+v", gotAdmitted, gotWaiting, wantAdmitted, wantWaiting)
	}
	if len(records) == 0 {
		t.Fatalf("RED: no shadow admission recorded; router-todo is held by import alone")
	}
	for _, r := range records {
		if r.HeldBy == "" {
			t.Errorf("RED: record %+v has no holder", r)
		}
		if r.ID == "core-two" {
			t.Errorf("RED: core-two edits the package of core-lane, the new rule holds it too, but it was recorded: %+v", r)
		}
	}
	if records[0].ID != "router-todo" || records[0].HeldBy != "core-lane" {
		t.Errorf("RED: first record = %+v, want router-todo held by core-lane", records[0])
	}
}

func TestC1995_004_ThePolicyAdmissionStageDefaultsToShadowAndRefusesAnUnknownWord(t *testing.T) {
	p, err := policy.Parse([]byte(`{"fleet":{"orchestrator":{"stage":"enforce"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.FleetConfig().Orchestrator.Admission; got != "shadow" {
		t.Errorf("RED: Admission = %q, want the default shadow", got)
	}
	p, err = policy.Parse([]byte(`{"fleet":{"orchestrator":{"admission":"enforce"}}}`))
	if err != nil {
		t.Fatalf("RED: admission enforce refused: %v", err)
	}
	if got := p.FleetConfig().Orchestrator.Admission; got != "enforce" {
		t.Errorf("RED: Admission = %q, want enforce", got)
	}
	if _, err := policy.Parse([]byte(`{"fleet":{"orchestrator":{"admission":"enfroce"}}}`)); err == nil || !strings.Contains(err.Error(), `"enfroce" is not "shadow" or "enforce"`) {
		t.Errorf("RED: err = %v, want the unknown admission stage refused by name", err)
	}
}

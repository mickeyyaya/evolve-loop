package main

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func goListOf(pkgs map[string][]string, dirs *[]string, code int) func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
	return func(ctx context.Context, _ string, dir string, _ []string, _ []string, _ io.Reader, stdout, stderr io.Writer) (int, error) {
		if _, bounded := ctx.Deadline(); !bounded {
			return -1, context.DeadlineExceeded
		}
		*dirs = append(*dirs, dir)
		for pkg, deps := range pkgs {
			full := []string{}
			for _, d := range deps {
				full = append(full, admission.ModulePath+"/"+d)
			}
			io.WriteString(stdout, admission.ModulePath+"/"+pkg+"|"+strings.Join(full, " ")+"\n")
		}
		io.WriteString(stderr, "go: cannot load")
		return code, nil
	}
}

type graphAsk struct {
	cfg  policy.OrchestratorConfig
	wave int
}

func listingOf(pkgs map[string][]string) string {
	var out strings.Builder
	for pkg, deps := range pkgs {
		full := []string{}
		for _, d := range deps {
			full = append(full, admission.ModulePath+"/"+d)
		}
		out.WriteString(admission.ModulePath + "/" + pkg + "|" + strings.Join(full, " ") + "\n")
	}
	return out.String()
}

func TestAdmissionShadow_AdmitsThePlannedTodosOverTheGraphOfTheWave(t *testing.T) {
	t.Parallel()
	listing := listingOf(map[string][]string{"internal/sysexec": nil, "internal/inboxrank": {"internal/sysexec"}, "internal/fleet": {"internal/sysexec", "internal/inboxrank"}, "cmd/evolve": {"internal/fleet"}})
	var asked []graphAsk
	fn := admissionShadow(func(oc policy.OrchestratorConfig, wave int) (*admission.ModuleGraph, error) {
		asked = append(asked, graphAsk{cfg: oc, wave: wave})
		return admission.NewModuleGraph(listing, oc.HubMinImporters)
	})
	todos := []fleet.Todo{
		{ID: "a", Files: []string{"go/internal/inboxrank/a.go"}},
		{ID: "f", Files: []string{"go/internal/fleet/pool.go"}},
		{ID: "n"},
	}
	text := func(id string) string { return map[string]string{"n": "fix go/internal/sysexec"}[id] }
	cfg := policy.OrchestratorConfig{Stage: "shadow", ScanK: 32, HubMinImporters: 2}

	got, err := fn(9, todos, text, cfg)

	if err != nil {
		t.Fatal(err)
	}
	want := loopwave.AdmissionShadow{Admitted: 2, Waits: []loopwave.AdmissionWait{{ID: "f", Rank: 1, Cells: []string{"internal/fleet"}, BlockedBy: "a"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("shadow = %+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(asked, []graphAsk{{cfg: cfg, wave: 9}}) {
		t.Errorf("graph asked %+v, want one ask for wave 9 with the orchestrator block", asked)
	}
}

func TestAdmissionShadow_AtTheEnforceAdmissionKeyTheSharedEditRuleCounts(t *testing.T) {
	t.Parallel()
	listing := listingOf(map[string][]string{"internal/sysexec": nil, "internal/inboxrank": {"internal/sysexec"}, "internal/fleet": {"internal/sysexec", "internal/inboxrank"}, "cmd/evolve": {"internal/fleet"}})
	fn := admissionShadow(func(oc policy.OrchestratorConfig, _ int) (*admission.ModuleGraph, error) {
		return admission.NewModuleGraph(listing, oc.HubMinImporters)
	})
	todos := []fleet.Todo{{ID: "a", Files: []string{"go/internal/inboxrank/a.go"}}, {ID: "f", Files: []string{"go/internal/fleet/pool.go"}}}
	for admissionKey, want := range map[string]int{"shadow": 1, "enforce": 2} {
		cfg := policy.OrchestratorConfig{Stage: "shadow", Admission: admissionKey, ScanK: 32, HubMinImporters: 2}

		got, err := fn(9, todos, func(string) string { return "" }, cfg)

		if err != nil {
			t.Fatal(err)
		}
		if got.Admitted != want {
			t.Errorf("admission %s: admitted = %d, want %d: f imports the inboxrank that a edits but edits no package of a", admissionKey, got.Admitted, want)
		}
	}
}

func TestAdmissionShadow_ReportsAGraphFailure(t *testing.T) {
	t.Parallel()
	fn := admissionShadow(func(policy.OrchestratorConfig, int) (*admission.ModuleGraph, error) {
		return nil, errors.New("go: cannot load")
	})
	if _, err := fn(1, nil, func(string) string { return "" }, policy.OrchestratorConfig{ScanK: 32, HubMinImporters: 120}); err == nil || !strings.Contains(err.Error(), "go: cannot load") {
		t.Fatalf("err = %v, want the graph failure", err)
	}
}

func TestGraphLoaderOver_RunsOneBoundedGoListInTheGoDirOfTheRoot(t *testing.T) {
	t.Parallel()
	var dirs []string
	graph := map[string][]string{"internal/sysexec": nil, "internal/inboxrank": {"internal/sysexec"}}

	got, err := graphLoaderOver(goListOf(graph, &dirs, 0))(context.Background(), "/plane", 2)

	if err != nil || got == nil || len(got.Packages([]string{"go/internal/inboxrank/a.go"})) != 1 {
		t.Fatalf("graph = %v, %v, want the graph of the listing", got, err)
	}
	if !reflect.DeepEqual(dirs, []string{"/plane/go"}) {
		t.Errorf("go list ran in %v, want once in /plane/go", dirs)
	}
}

func TestGraphLoaderOver_ReportsAGoListFailure(t *testing.T) {
	t.Parallel()
	var dirs []string
	if _, err := graphLoaderOver(goListOf(nil, &dirs, 1))(context.Background(), "/plane", 120); err == nil || !strings.Contains(err.Error(), "go: cannot load") {
		t.Fatalf("err = %v, want the go list failure", err)
	}
}

func TestWiredWave_WiresTheAdmissionShadow(t *testing.T) {
	t.Parallel()
	b := &loopBatchCoordinator{ctx: context.Background(), cfg: loopConfig{ProjectRoot: t.TempDir()}, stderr: io.Discard}
	if !b.wiredWave().AdmissionWired() {
		t.Fatal("the live wave engine must carry the admission shadow")
	}
	if nullWaveEngine(loopwave.Roots{}, io.Discard).AdmissionWired() {
		t.Error("the rootless engine of the pure facades runs no go list")
	}
}

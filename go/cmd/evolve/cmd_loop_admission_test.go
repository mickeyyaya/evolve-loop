package main

import (
	"context"
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

func TestAdmissionShadow_AdmitsThePlannedTodosOverTheModuleGraph(t *testing.T) {
	t.Parallel()
	var dirs []string
	graph := map[string][]string{"internal/sysexec": nil, "internal/inboxrank": {"internal/sysexec"}, "internal/fleet": {"internal/sysexec", "internal/inboxrank"}}
	fn := admissionShadow(context.Background(), "/plane/go", goListOf(graph, &dirs, 0))
	todos := []fleet.Todo{
		{ID: "a", Files: []string{"go/internal/inboxrank/a.go"}},
		{ID: "f", Files: []string{"go/internal/fleet/pool.go"}},
		{ID: "n"},
	}
	text := func(id string) string { return map[string]string{"n": "fix go/internal/sysexec"}[id] }
	got, err := fn(todos, text, policy.OrchestratorConfig{Stage: "shadow", ScanK: 32, HubMinImporters: 2})
	if err != nil {
		t.Fatal(err)
	}
	want := loopwave.AdmissionShadow{Admitted: 2, Waits: []loopwave.AdmissionWait{{ID: "f", Rank: 1, Cells: []string{"internal/fleet"}, BlockedBy: "a"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("shadow = %+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(dirs, []string{"/plane/go"}) {
		t.Errorf("go list ran in %v, want once in /plane/go", dirs)
	}
}

func TestAdmissionShadow_ReportsAGraphFailure(t *testing.T) {
	t.Parallel()
	var dirs []string
	fn := admissionShadow(context.Background(), "/plane/go", goListOf(nil, &dirs, 1))
	if _, err := fn(nil, func(string) string { return "" }, policy.OrchestratorConfig{ScanK: 32, HubMinImporters: 120}); err == nil || !strings.Contains(err.Error(), "go: cannot load") {
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

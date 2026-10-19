package admission_test

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

var testModule = map[string][]string{
	"internal/sysexec":    nil,
	"internal/log":        nil,
	"internal/inboxrank":  {"internal/sysexec"},
	"internal/quotastate": {"internal/sysexec"},
	"internal/fleet":      {"internal/sysexec", "internal/inboxrank"},
	"internal/policy":     {"internal/sysexec", "internal/log"},
}

var importerOfLeaves = map[string][]string{
	"cmd/evolve": {"internal/fleet", "internal/policy", "internal/quotastate"},
}

const testHubMin = 4

func listing(module map[string][]string) string {
	var b strings.Builder
	for pkg, deps := range module {
		full := make([]string, 0, len(deps)+1)
		for _, d := range deps {
			full = append(full, admission.ModulePath+"/"+d)
		}
		full = append(full, "fmt", "strings")
		b.WriteString(admission.ModulePath + "/" + pkg + "|" + strings.Join(full, " ") + "\n")
	}
	return b.String()
}

func testGraph(t *testing.T) *admission.ModuleGraph {
	t.Helper()
	g, err := admission.NewModuleGraph(listing(testModule), testHubMin)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func admissionGraph(t *testing.T) *admission.ModuleGraph {
	t.Helper()
	module := map[string][]string{}
	for _, m := range []map[string][]string{testModule, importerOfLeaves} {
		for pkg, deps := range m {
			module[pkg] = deps
		}
	}
	g, err := admission.NewModuleGraph(listing(module), testHubMin)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func admitter(t *testing.T, text map[string]string) admission.Admitter {
	return admission.Admitter{Graph: admissionGraph(t), Text: func(id string) string { return text[id] }}
}

func todo(id string, files ...string) fleet.Todo { return fleet.Todo{ID: id, Files: files} }

func ids(ds []admission.Decision) []string {
	out := []string{}
	for _, d := range ds {
		out = append(out, d.ID)
	}
	return out
}

func TestAdmit_ATodoInABusyCellWaits(t *testing.T) {
	a := admitter(t, nil)
	held := []admission.Holding{{Lane: "lane-1", Todo: todo("a", "go/internal/inboxrank/inboxrank.go")}}
	admitted, waiting := a.Admit(held, []fleet.Todo{todo("d", "go/internal/inboxrank/explain.go"), todo("b", "go/internal/quotastate/windows.go")}, 32)
	if got := ids(admitted); !reflect.DeepEqual(got, []string{"b"}) {
		t.Fatalf("admitted = %v, want [b]: b has a free cell", got)
	}
	want := admission.Decision{ID: "d", Rank: 0, Cells: []string{"internal/inboxrank"}, BlockedBy: "lane-1"}
	if len(waiting) != 1 || !reflect.DeepEqual(waiting[0], want) {
		t.Fatalf("waiting = %+v, want [%+v]: d shares inboxrank with lane-1 and keeps its rank", waiting, want)
	}
	if admitted[0].Rank != 1 || !reflect.DeepEqual(admitted[0].Cells, []string{"internal/quotastate"}) || admitted[0].BlockedBy != "" {
		t.Errorf("admitted b = %+v, want rank 1, cell quotastate, no blocker", admitted[0])
	}
}

func TestAdmit_TheModuleRootFilesMapToTheHubCell(t *testing.T) {
	for _, file := range []string{"go/go.mod", "go/go.sum", "go/Makefile"} {
		t.Run(file, func(t *testing.T) {
			a := admitter(t, nil)
			admitted, waiting := a.Admit(nil, []fleet.Todo{todo("first", file), todo("second", file)}, 32)
			if got := ids(admitted); !reflect.DeepEqual(got, []string{"first"}) || admitted[0].Cells[0] != admission.HubCell {
				t.Fatalf("admitted = %+v, want first alone in the hub cell", admitted)
			}
			if got := ids(waiting); !reflect.DeepEqual(got, []string{"second"}) || waiting[0].BlockedBy != "first" {
				t.Fatalf("waiting = %+v, want second held behind first", waiting)
			}
		})
	}
}

func TestAdmit_FilesOutsideGoMapToNoCellAndConflictWithNothing(t *testing.T) {
	a := admitter(t, nil)
	candidates := []fleet.Todo{todo("docs", "docs/a.md", "scripts/x.sh"), todo("docs2", "docs/a.md"), todo("mixed", "go/internal/sysexec/README.txt", "go/notes.txt")}
	admitted, waiting := a.Admit(nil, candidates, 32)
	if got := ids(admitted); !reflect.DeepEqual(got, []string{"docs", "docs2", "mixed"}) || len(waiting) != 0 {
		t.Fatalf("admitted = %v, waiting = %v, want every non-Go todo admitted: known limit, no cell to conflict on", got, ids(waiting))
	}
	if len(admitted[0].Cells) != 0 {
		t.Errorf("cells = %v, want none", admitted[0].Cells)
	}
}

func TestAdmit_TheGlobalCellAdmitsOne(t *testing.T) {
	a := admitter(t, nil)
	candidates := []fleet.Todo{
		todo("e1", "go/internal/sysexec/command.go"),
		todo("e2", "go/go.mod"),
		todo("e3", ".evolve/policy.json"),
		todo("e4"),
		todo("e5", "go.sum"),
	}
	admitted, waiting := a.Admit(nil, candidates, 32)
	if got := ids(admitted); !reflect.DeepEqual(got, []string{"e1"}) {
		t.Fatalf("admitted = %v, want [e1]: the hub cell has capacity 1", got)
	}
	if got := ids(waiting); !reflect.DeepEqual(got, []string{"e2", "e3", "e4", "e5"}) {
		t.Fatalf("waiting = %v, want [e2 e3 e4 e5]", got)
	}
	for _, w := range waiting {
		if w.BlockedBy != "e1" || w.Cells[0] != admission.HubCell {
			t.Errorf("%s = %+v, want the hub cell, blocked by e1", w.ID, w)
		}
	}
	if !reflect.DeepEqual(admitted[0].Cells, []string{admission.HubCell, "internal/sysexec"}) {
		t.Errorf("e1 cells = %v, want [hub internal/sysexec]", admitted[0].Cells)
	}
}

func TestAdmit_ATodoWithNoFilesTakesItsPackageCell(t *testing.T) {
	a := admitter(t, map[string]string{
		"n": "Rank ties in go/internal/inboxrank break by age",
		"m": "the window drifts in go/internal/quotastate/windows.go.",
		"x": "a note that names go/internal/nosuchpkg only",
	})
	held := []admission.Holding{{Lane: "lane-1", Todo: todo("a", "go/internal/inboxrank/inboxrank.go")}}
	admitted, waiting := a.Admit(held, []fleet.Todo{todo("n"), todo("m", "m"), todo("x")}, 32)
	if got := ids(waiting); !reflect.DeepEqual(got, []string{"n"}) || !reflect.DeepEqual(waiting[0].Cells, []string{"internal/inboxrank"}) {
		t.Fatalf("waiting = %+v, want n in the inboxrank cell that its title names", waiting)
	}
	if got := ids(admitted); !reflect.DeepEqual(got, []string{"m", "x"}) {
		t.Fatalf("admitted = %v, want [m x]", got)
	}
	if !reflect.DeepEqual(admitted[0].Cells, []string{"internal/quotastate"}) {
		t.Errorf("m (its id as its only file) cells = %v, want the quotastate cell that its text names", admitted[0].Cells)
	}
	if !reflect.DeepEqual(admitted[1].Cells, []string{admission.HubCell}) {
		t.Errorf("x names no known package, cells = %v, want the hub cell", admitted[1].Cells)
	}
}

func TestAdmit_ATodoOutsideGoIsAlwaysAdmitted(t *testing.T) {
	a := admitter(t, nil)
	held := []admission.Holding{
		{Lane: "lane-1", Todo: todo("e", "go/internal/sysexec/command.go")},
		{Lane: "lane-2", Todo: todo("a", "go/internal/inboxrank/inboxrank.go")},
	}
	admitted, waiting := a.Admit(held, []fleet.Todo{todo("f", "docs/operations/wave-operations.md", "skills/docs-write/SKILL.md")}, 32)
	if len(waiting) != 0 || len(admitted) != 1 || len(admitted[0].Cells) != 0 {
		t.Fatalf("admitted = %+v, waiting = %+v: a todo outside go/ holds no cell", admitted, waiting)
	}
}

func TestAdmit_ImportingAHubNeverConflicts(t *testing.T) {
	a := admitter(t, nil)
	held := []admission.Holding{{Lane: "lane-1", Todo: todo("e", "go/internal/sysexec/command.go")}}
	admitted, waiting := a.Admit(held, []fleet.Todo{todo("q", "go/internal/quotastate/windows.go"), todo("p", "go/internal/policy/fleet.go")}, 32)
	if got := ids(admitted); len(waiting) != 0 || !reflect.DeepEqual(got, []string{"q", "p"}) {
		t.Fatalf("admitted = %v, waiting = %+v: quotastate and policy import the hub sysexec, which never conflicts", got, waiting)
	}
}

func TestAdmit_ALeafLaneHoldsNoReachOverAnUnrelatedPackage(t *testing.T) {
	a := admitter(t, nil)
	held := []admission.Holding{{Lane: "lane-1", Todo: todo("c", "go/cmd/evolve/main.go")}}
	admitted, waiting := a.Admit(held, []fleet.Todo{todo("i", "go/internal/inboxrank/inboxrank.go")}, 32)
	if got := ids(admitted); len(waiting) != 0 || !reflect.DeepEqual(got, []string{"i"}) {
		t.Fatalf("admitted = %v, waiting = %+v: cmd/evolve is a leaf, so it holds no reach over inboxrank", got, waiting)
	}
}

func TestAdmit_AnEditedPackageThatTheOtherImportsConflicts(t *testing.T) {
	a := admitter(t, nil)
	_, waiting := a.Admit([]admission.Holding{{Lane: "lane-1", Todo: todo("i", "go/internal/inboxrank/inboxrank.go")}},
		[]fleet.Todo{todo("f", "go/internal/fleet/pool.go")}, 32)
	if len(waiting) != 1 || waiting[0].BlockedBy != "lane-1" {
		t.Fatalf("waiting = %+v: fleet imports the inboxrank that lane-1 edits", waiting)
	}
	_, waiting = a.Admit([]admission.Holding{{Lane: "lane-2", Todo: todo("f", "go/internal/fleet/pool.go")}},
		[]fleet.Todo{todo("i", "go/internal/inboxrank/inboxrank.go")}, 32)
	if len(waiting) != 1 || waiting[0].BlockedBy != "lane-2" {
		t.Fatalf("waiting = %+v: inboxrank is imported by the fleet that lane-2 edits", waiting)
	}
}

func TestAdmit_TheScanStopsAtScanK(t *testing.T) {
	a := admitter(t, nil)
	candidates := []fleet.Todo{
		todo("q", "go/internal/quotastate/windows.go"),
		todo("q2", "go/internal/quotastate/state.go"),
		todo("i", "go/internal/inboxrank/inboxrank.go"),
		todo("p", "go/internal/policy/fleet.go"),
	}
	admitted, waiting := a.Admit(nil, candidates, 2)
	if got := ids(admitted); !reflect.DeepEqual(got, []string{"q", "i"}) {
		t.Fatalf("admitted = %v, want [q i]: the scan stops after 2 admissible todos", got)
	}
	if got := ids(waiting); !reflect.DeepEqual(got, []string{"q2"}) || waiting[0].Rank != 1 {
		t.Fatalf("waiting = %+v, want q2 at rank 1 and p unscanned", waiting)
	}
	if admitted, _ := a.Admit(nil, candidates, 0); len(admitted) != 0 {
		t.Errorf("scan_k 0 admits nothing, got %v", ids(admitted))
	}
}

func TestAdmit_ANilTextReadsAsEmpty(t *testing.T) {
	a := admission.Admitter{Graph: admissionGraph(t)}
	admitted, _ := a.Admit(nil, []fleet.Todo{todo("n")}, 32)
	if len(admitted) != 1 || !reflect.DeepEqual(admitted[0].Cells, []string{admission.HubCell}) {
		t.Fatalf("admitted = %+v: no text names no package, so the todo takes the hub cell", admitted)
	}
}

var propertyFiles = []string{
	"go/internal/sysexec/command.go",
	"go/internal/log/log.go",
	"go/internal/inboxrank/inboxrank.go",
	"go/internal/quotastate/windows.go",
	"go/internal/fleet/pool.go",
	"go/internal/policy/fleet.go",
	"go/go.mod",
	"go/go.sum",
	"go/Makefile",
	".evolve/policy.json",
	"docs/a.md",
	"README.md",
}

func oracleFootprint(files []string) (edits map[string]bool, hub bool) {
	edits = map[string]bool{}
	if len(files) == 0 {
		return edits, true
	}
	for _, f := range files {
		switch {
		case f == "go/go.mod" || f == "go/go.sum" || f == "go/Makefile" || f == ".evolve/policy.json":
			hub = true
		case strings.HasPrefix(f, "go/internal/"):
			pkg := f[len("go/"):strings.LastIndex(f, "/")]
			edits[pkg] = true
			if pkg == "internal/sysexec" {
				hub = true
			}
		}
	}
	return edits, hub
}

func oracleReaches(edits map[string]bool, target map[string]bool) bool {
	for pkg := range edits {
		for _, dep := range testModule[pkg] {
			if dep != "internal/sysexec" && target[dep] {
				return true
			}
		}
	}
	return false
}

func oracleConflict(a, b []string) bool {
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
	return oracleReaches(ea, eb) || oracleReaches(eb, ea)
}

func randomTodo(r *rand.Rand, id string) fleet.Todo {
	n := r.Intn(3)
	files := make([]string, 0, n)
	for range n {
		files = append(files, propertyFiles[r.Intn(len(propertyFiles))])
	}
	return todo(id, files...)
}

func TestAdmit_TwoAdmittedTodosNeverConflict(t *testing.T) {
	a := admitter(t, nil)
	r := rand.New(rand.NewSource(20261017))
	for round := range 500 {
		var held []admission.Holding
		var candidates []fleet.Todo
		files := map[string][]string{}
		for i := range r.Intn(3) {
			td := randomTodo(r, "h"+string(rune('a'+i)))
			files[td.ID] = td.Files
			if !conflictsWithAny(held, td.Files, files) {
				held = append(held, admission.Holding{Lane: td.ID, Todo: td})
			}
		}
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
				if oracleConflict(files[d.ID], files[other]) {
					t.Fatalf("round %d: admitted %s %v conflicts with %s %v", round, d.ID, files[d.ID], other, files[other])
				}
			}
			running = append(running, d.ID)
		}
		for _, w := range waiting {
			if !oracleConflict(files[w.ID], files[w.BlockedBy]) {
				t.Fatalf("round %d: %s %v waits on %s %v, which does not conflict", round, w.ID, files[w.ID], w.BlockedBy, files[w.BlockedBy])
			}
		}
	}
}

func conflictsWithAny(held []admission.Holding, f []string, files map[string][]string) bool {
	for _, h := range held {
		if oracleConflict(f, files[h.Lane]) {
			return true
		}
	}
	return false
}

package overlap

import (
	"path"
	"reflect"
	"strings"
	"testing"
)

const testModule = "example.com/m"

func pkg(name string, deps ...string) Package {
	full := make([]string, 0, len(deps))
	for _, d := range deps {
		full = append(full, testModule+"/internal/"+d)
	}
	return Package{
		ImportPath: testModule + "/internal/" + name,
		Dir:        "go/internal/" + name,
		Files:      []string{"go/internal/" + name + "/" + name + ".go", "go/internal/" + name + "/" + name + "_test.go"},
		Deps:       full,
	}
}

func ip(name string) string { return testModule + "/internal/" + name }

func src(name string) string { return "go/internal/" + name + "/" + name + ".go" }

func testModuleMap() Module {
	a := pkg("a")
	a.Files = append(a.Files, "go/internal/a/assets/x.txt")
	return Module{
		Path: testModule,
		Packages: []Package{
			a,
			pkg("b", "leaf"),
			pkg("c", "leaf"),
			pkg("leaf"),
			pkg("top", "a"),
			pkg("flagregistry"),
		},
	}
}

type fakeCatalogs struct {
	derived    map[Side]map[string]bool
	dataReads  map[string]bool
	fired      []string
	onConflict []string
}

func (fakeCatalogs) Bookkeeping(p string) bool {
	return strings.HasPrefix(p, ".evolve/inbox/") ||
		strings.HasPrefix(p, "knowledge-base/cycles/cycle-") ||
		strings.HasPrefix(p, "go/acs/cycle")
}

func (fakeCatalogs) BuildZone(p string) bool {
	base := path.Base(p)
	return p == "go/go.mod" || p == "go/go.sum" || strings.HasPrefix(p, "go/vendor/") ||
		p == ".evolve/policy.json" || base == ".gitattributes" || base == ".gitignore"
}

func (fakeCatalogs) GateZone(p string) bool {
	return p == "go/Makefile" || p == "go/.cover-strict" || p == "go/.apicover-enforce" ||
		strings.HasPrefix(p, ".github/workflows/")
}

func (f fakeCatalogs) DerivedOutput(side Side, p string) bool { return f.derived[side][p] }

func (f fakeCatalogs) DataRead(p string) bool { return f.dataReads[p] }

func (f fakeCatalogs) Fired(_, _, conflicted []string) []string {
	if f.onConflict != nil && !reflect.DeepEqual(conflicted, f.onConflict) {
		return nil
	}
	return f.fired
}

func baseInput(lane, peer []string) Input {
	return Input{
		Lane:     lane,
		Peer:     peer,
		Merge:    MergeClean,
		Module:   testModuleMap(),
		Catalogs: fakeCatalogs{},
	}
}

func assertTier(t *testing.T, got Proof, tier Tier, rules ...Rule) {
	t.Helper()
	if got.Tier != tier || !reflect.DeepEqual(got.Rules, rules) {
		t.Fatalf("tier = %s rules = %v, want %s %v (evidence %+v)", got.Tier, got.Rules, tier, rules, got.Evidence)
	}
}

func assertStrings(t *testing.T, field string, got []string, want ...string) {
	t.Helper()
	if want == nil {
		want = []string{}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %q, want %q", field, got, want)
	}
}

func assertEdges(t *testing.T, field string, got []Edge, want ...Edge) {
	t.Helper()
	if want == nil {
		want = []Edge{}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %v, want %v", field, got, want)
	}
}

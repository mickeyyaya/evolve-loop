package gatehermetic

import (
	"context"
	"errors"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/goload"
)

func namedForTest() {}

func TestRootOf_NamesAFunctionAndRefusesAClosureAMethodAndANonFunction(t *testing.T) {
	t.Parallel()
	got, err := RootOf(namedForTest)
	if err != nil || got != (Root{Package: modulePath + "/internal/gatehermetic", Func: "namedForTest"}) {
		t.Errorf("RootOf(named) = %+v, %v", got, err)
	}
	if got, err := RootOf(strings.ToUpper); err != nil || got != (Root{Package: "strings", Func: "ToUpper"}) {
		t.Errorf("RootOf(stdlib) = %+v, %v", got, err)
	}
	var nilFunc func()
	for name, fn := range map[string]any{
		"closure": func() {},
		"method":  (*pkgInfo).excluded,
		"value":   42,
		"nil":     nilFunc,
	} {
		if _, err := RootOf(fn); err == nil {
			t.Errorf("RootOf(%s) gave no error", name)
		}
	}
}

func TestLoad_FailsClosedOnEachBrokenLoad(t *testing.T) {
	t.Parallel()
	root := gateRoot("gates", "Run")
	broken := gatesFile(`"os"`, "func Run() { var x int = \"s\"; _ = x }\n")
	bodyless := gatesFile("", "func Run()\n")
	good := gatesFile("", "func Run() {}\n")
	cases := map[string]struct {
		files map[string]string
		root  Root
		want  string
	}{
		"type error":         {broken, root, "does not type-check"},
		"undeclared root":    {good, gateRoot("gates", "Nope"), "does not declare the root"},
		"undeclared package": {good, gateRoot("absent", "Run"), "go list"},
		"bodyless root":      {bodyless, root, "has no body"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g, err := Load(context.Background(), realLoader, Source{ModuleDir: fixtureDir(t, tc.files), Roots: []Root{tc.root}})
			if err == nil || !strings.Contains(err.Error(), tc.want) || len(g.roots) != 0 {
				t.Errorf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestLoad_ReportsARunnerFailureAndAnEmptySource(t *testing.T) {
	t.Parallel()
	src := Source{ModuleDir: ".", Roots: []Root{gateRoot("gates", "Run")}}
	loader := realLoader
	loader.Run = replying("", 1, nil)
	if _, err := Load(context.Background(), loader, src); err == nil || !strings.Contains(err.Error(), "exit 1") {
		t.Errorf("exit code: err = %v", err)
	}
	loader.Run = replying("", 0, errors.New("boom"))
	if _, err := Load(context.Background(), loader, src); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("runner error: err = %v", err)
	}
	if _, err := Load(context.Background(), realLoader, Source{ModuleDir: "."}); err == nil {
		t.Error("an empty source loaded")
	}
}

func TestLoad_UnknownPackageOfAGraphGivesAnAbsentAudit(t *testing.T) {
	t.Parallel()
	g := loadFixture(t, gatesFile("", "func Run() {}\n"), gateRoot("gates", "Run"))

	got := Audit(g, "no/such/pkg", Contract())

	if len(got) != 1 || got[0].Class != "absent" {
		t.Errorf("audit = %+v", got)
	}
}

func TestLoad_RecordsEachFormOfAssignmentToAPackageVariable(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"internal/vars/vars.go": `package vars

var A, B, C, D, E, F, G, H, I int

var Arr [2]int

var P = &H

type St struct{ X int }

var S St

var X, Y, Z func()
`,
		"internal/gates/gates.go": `package gates

import "` + modulePath + `/internal/vars"

func get() *vars.St { return &vars.S }

func Run() {
	vars.A = 1
	(vars.B) = 2
	*vars.P = 3
	vars.C++
	for vars.D = range 3 {
	}
	_ = vars.Arr[:]
	_ = &vars.Arr
	vars.S = vars.St{}
	vars.S.X = 4
	*get() = vars.St{}
	x := 1
	x = 2
	_ = x
	_ = &vars.E
	vars.X = nil
}
`,
	}

	g := loadFixture(t, files, gateRoot("gates", "Run"))

	for _, name := range []string{"A", "B", "C", "D", "E", "Arr", "S", "X"} {
		if !g.assigned[modulePath+"/internal/vars.var "+name] {
			t.Errorf("%s is not recorded as assigned: %v", name, g.assigned)
		}
	}
	if g.assigned[modulePath+"/internal/vars.var F"] || g.assigned[modulePath+"/internal/vars.var Y"] {
		t.Errorf("assigned = %v", g.assigned)
	}
}

func TestLoad_ARootInTheStdlibIsAnError(t *testing.T) {
	t.Parallel()
	_, err := Load(context.Background(), realLoader, Source{ModuleDir: ".", Roots: []Root{{Package: "strings", Func: "ToUpper"}}})
	if err == nil || !strings.Contains(err.Error(), "no package strings") {
		t.Errorf("err = %v", err)
	}
}

func TestLoad_ABrokenThirdPartyPackageIsAnErrorAndAGoodOneIsWalked(t *testing.T) {
	t.Parallel()
	mod := func(dep1, dep2 string) map[string]string {
		return map[string]string{
			"go.mod":                  "module " + modulePath + "\n\ngo 1.23\n\nrequire (\n\texample.com/one v0.0.0\n\texample.com/two v0.0.0\n)\n\nreplace example.com/one => ./one\n\nreplace example.com/two => ./two\n",
			"one/go.mod":              "module example.com/one\n\ngo 1.23\n",
			"one/one.go":              dep1,
			"two/go.mod":              "module example.com/two\n\ngo 1.23\n\nrequire example.com/one v0.0.0\n\nreplace example.com/one => ../one\n",
			"two/two.go":              dep2,
			"internal/gates/gates.go": "package gates\n\nimport \"example.com/two\"\n\nfunc Run() int { return two.F() }\n",
		}
	}
	good1 := "package one\n\nimport \"unsafe\"\n\nfunc G() int { return int(unsafe.Sizeof(0)) }\n"
	good2 := "package two\n\nimport \"example.com/one\"\n\nfunc F() int { return one.G() }\n"

	if g := loadFixture(t, mod(good1, good2), gateRoot("gates", "Run")); g.pkgs["example.com/one"] == nil || g.pkgs["example.com/two"] == nil {
		t.Errorf("third-party packages = %v", g.pkgs)
	}
	_, err := Load(context.Background(), realLoader, Source{ModuleDir: fixtureDir(t, mod("package one\n\nfunc G() int { return \"s\" }\n", good2)), Roots: []Root{gateRoot("gates", "Run")}})
	if err == nil || !strings.Contains(err.Error(), "example.com/one does not type-check") {
		t.Errorf("err = %v", err)
	}
}

func TestRecheck_FailsOnAnUnreadableDirectoryAParseErrorAndAnUnknownImport(t *testing.T) {
	t.Parallel()
	prog := func() goload.Program {
		return goload.Program{Fset: token.NewFileSet(), Packages: map[string]*goload.Package{}}
	}
	cases := map[string]map[string]string{
		"two packages":   {"a.go": "package a\n", "b.go": "package b\n"},
		"parse error":    {"a.go": "package a\n\nfunc (\n"},
		"unknown import": {"a.go": "package a\n\nimport _ \"nowhere/pkg\"\n"},
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := writeModule(t, files)

			got, err := recheck(prog(), &goload.Package{Path: "x", Dir: dir}, map[string]*types.Package{})

			if err == nil || got != nil {
				t.Errorf("got = %+v err = %v", got, err)
			}
		})
	}
}

func TestHoldsCode_SeesFunctionsInterfacesAndChannelsAtAnyDepth(t *testing.T) {
	t.Parallel()
	fn := types.NewSignatureType(nil, nil, nil, nil, nil, false)
	plain := types.Typ[types.Int]
	named := types.NewNamed(types.NewTypeName(token.NoPos, nil, "N", nil), plain, nil)
	alias := types.NewAlias(types.NewTypeName(token.NoPos, nil, "Al", nil), fn)
	field := func(t types.Type) *types.Struct {
		return types.NewStruct([]*types.Var{types.NewField(token.NoPos, nil, "f", t, false)}, nil)
	}
	loop := types.NewNamed(types.NewTypeName(token.NoPos, nil, "L", nil), nil, nil)
	loop.SetUnderlying(field(types.NewPointer(loop)))
	cases := map[string]struct {
		t    types.Type
		want bool
	}{
		"signature": {fn, true}, "interface": {types.NewInterfaceType(nil, nil), true},
		"chan": {types.NewChan(types.SendRecv, plain), true}, "alias": {alias, true},
		"pointer": {types.NewPointer(fn), true}, "slice": {types.NewSlice(fn), true},
		"array": {types.NewArray(fn, 2), true}, "map key": {types.NewMap(fn, plain), true},
		"map elem": {types.NewMap(plain, fn), true}, "struct": {field(fn), true},
		"named plain": {named, false}, "struct plain": {field(plain), false}, "loop": {loop, false},
	}
	for name, tc := range cases {
		if got := holdsCode(tc.t, map[types.Type]bool{}); got != tc.want {
			t.Errorf("%s = %v, want %v", name, got, tc.want)
		}
	}
}

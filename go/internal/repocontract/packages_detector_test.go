package repocontract

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"
)

func TestClimbsOutOfItsPackage(t *testing.T) {
	cases := []struct {
		name, dir, call string
		want            bool
	}{
		{"to the module root", "internal/p", `filepath.Abs("../..")`, true},
		{"to the module root in one literal per step", "internal/a/b", `filepath.Join("..", "..", "..")`, true},
		{"anchored on the file, then a subpath", "internal/p", `filepath.Join(filepath.Dir(self), "..", "..", "acs")`, true},
		{"onto a top-level directory", "internal/p", `walk("..", vocab)`, true},
		{"into a sibling's testdata", "internal/p", `filepath.Join("..", "testdata", "x.json")`, false},
		{"onto a top-level directory, then a name", "internal/p", `filepath.Join("..", name)`, false},
		{"one step inside a deep package", "internal/a/b", `walk("..")`, false},
		{"past the module root", "internal/p", `filepath.Abs("../../..")`, false},
		{"a name undone, then one step", "internal/p", `filepath.Join("x", "..", "..")`, true},
		{"a climb, a name, then back", "internal/p", `filepath.Join("..", "..", "x", "..")`, true},
		{"a ./.. climb", "internal/p", `filepath.Abs("./../..")`, true},
		{"an empty segment before the climb", "internal/p", `filepath.Join("", "..", "..")`, true},
		{"any call at the module root", ".", `t.Helper()`, false},
		{"no climb, one level below the root", "test", `t.Helper()`, false},
		{"anchored on another directory", "internal/p", `filepath.Join(gitdir, "..", "..")`, false},
		{"a string prefix test", "internal/p", `strings.HasPrefix(rel, "..")`, false},
		{"handed to a function that reads the tree", "internal/p", `PartitionGraph(todos, 2, "../..")`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "x_test.go", "package p\n\nfunc f() { _ = "+tc.call+" }\n", 0)
			if err != nil {
				t.Fatal(err)
			}
			if got := climbsOutOfItsPackage(file, tc.dir); got != tc.want {
				t.Errorf("climbsOutOfItsPackage(%s in %s) = %v, want %v", tc.call, tc.dir, got, tc.want)
			}
		})
	}
}

func TestWalksUpToGoMod(t *testing.T) {
	cases := []struct {
		name, body string
		want       bool
	}{
		{"a bare for loop naming go.mod", `for { if exists(join(dir, "go.mod")) { return }; dir = parent(dir) }`, true},
		{"a range loop writing go.mod fixtures", `for _, d := range dirs { write(join(d, "go.mod")) }`, false},
		{"a counted loop naming go.mod", `for i := 0; i < 3; i++ { write(join(dirs[i], "go.mod")) }`, false},
		{"a conditional loop writing go.mod fixtures", `for len(ds) > 0 { write(join(ds[0], "go.mod")); ds = ds[1:] }`, false},
		{"go.mod written outside any loop", `write(join(dir, "go.mod")); for i := 0; i < 3; i++ { use(i) }`, false},
		{"a loop naming another file", `for { if exists(join(dir, "go.sum")) { return } }`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "x_test.go", "package p\n\nfunc f() {\n"+tc.body+"\n}\n", 0)
			if err != nil {
				t.Fatal(err)
			}
			if got := walksUpToGoMod(file); got != tc.want {
				t.Errorf("walksUpToGoMod(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestPackProblems(t *testing.T) {
	pack := []string{"./internal/in/..."}
	cases := []struct {
		name       string
		reading    map[string][]string
		selections []TestSelection
		want       []string
	}{
		{"in the pack", map[string][]string{"internal/in": {"TestA"}}, nil, nil},
		{"selected by name", map[string][]string{"internal/big": {"TestA", "TestB"}}, []TestSelection{{Package: "./internal/big", Tests: []string{"TestA", "TestB"}}}, nil},
		{"neither", map[string][]string{"internal/new": {"TestA"}}, nil, []string{"internal/new.TestA reads the whole tree"}},
		{"one test of a package left out", map[string][]string{"internal/big": {"TestA", "TestB"}}, []TestSelection{{Package: "./internal/big", Tests: []string{"TestA"}}}, []string{"internal/big.TestB reads the whole tree"}},
		{"a read no test reaches", map[string][]string{"internal/main": nil}, nil, []string{"internal/main reads the whole tree outside any test"}},
		{"a selection no longer reading", nil, []TestSelection{{Package: "./internal/gone", Tests: []string{"TestA"}}}, []string{"internal/gone.TestA is selected by name"}},
		{"selected and in the pack", map[string][]string{"internal/in": {"TestA"}}, []TestSelection{{Package: "./internal/in", Tests: []string{"TestA"}}}, []string{"internal/in.TestA is selected by name"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := packProblems(tc.reading, pack, tc.selections)
			if len(got) != len(tc.want) || !slices.EqualFunc(got, tc.want, strings.HasPrefix) {
				t.Errorf("packProblems = %q, want problems starting %q", got, tc.want)
			}
		})
	}
}

func TestReadingTestsOf_CreditsTheTestsThatReachARead(t *testing.T) {
	const source = `package p

var moduleRoot = filepath.Join("..", "..")

func sourcesMentioning(name string) []string { return walk(moduleRoot, name) }

func helperReachingTheRead() []string { return sourcesMentioning("New(") }

func TestSeam_OneConstructionSite(t *testing.T) { _ = helperReachingTheRead() }

func TestDirect(t *testing.T) { _ = filepath.Abs("../..") }

func TestLocal(t *testing.T) { _ = filepath.Join("testdata", "x.json") }

func TestMain(m *testing.M) { _ = moduleRoot }

func Testlowercase(t *testing.T) { _ = moduleRoot }
`
	file, err := parser.ParseFile(token.NewFileSet(), "x_test.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	tests, reads := readingTestsOf([]*ast.File{file}, "internal/p")
	if want := []string{"TestDirect", "TestSeam_OneConstructionSite"}; !reads || !slices.Equal(tests, want) {
		t.Fatalf("readingTestsOf = (%v, %v), want (%v, true): a test reaching a read through helpers and a package-level var is credited, TestMain and a non-test are not", tests, reads, want)
	}
	if tests, reads := readingTestsOf([]*ast.File{file}, "."); reads || len(tests) != 0 {
		t.Fatalf("at the module root nothing climbs out; got (%v, %v)", tests, reads)
	}
}

func TestTreeReadingTests_SelectsOnlyPackagesOutsideThePack(t *testing.T) {
	selections := TreeReadingTests()
	if len(selections) == 0 {
		t.Fatal("the pack selects no test by name")
	}
	for _, selection := range selections {
		if slices.ContainsFunc(Packages(), func(pattern string) bool { return strings.TrimSuffix(pattern, "/...") == selection.Package }) {
			t.Errorf("%s is run whole and by name", selection.Package)
		}
		if !strings.HasPrefix(selection.Package, "./") || len(selection.Tests) == 0 || !slices.IsSorted(selection.Tests) {
			t.Errorf("%s must be a ./ package pattern naming its tests sorted; got %v", selection.Package, selection.Tests)
		}
	}
}

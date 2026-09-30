package repocontract

import (
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
		name    string
		readers []string
		outside map[string]string
		want    []string
	}{
		{"in the pack", []string{"internal/in"}, nil, nil},
		{"recorded outside", []string{"internal/big"}, map[string]string{"internal/big": "slow"}, nil},
		{"neither", []string{"internal/new"}, nil, []string{"internal/new has a test that reads the whole tree"}},
		{"a record no longer found", nil, map[string]string{"internal/gone": "slow"}, []string{"internal/gone is recorded outside the pack"}},
		{"recorded and in the pack", []string{"internal/in"}, map[string]string{"internal/in": "slow"}, []string{"internal/in is recorded outside the pack"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := packProblems(tc.readers, pack, tc.outside)
			if len(got) != len(tc.want) || !slices.EqualFunc(got, tc.want, strings.HasPrefix) {
				t.Errorf("packProblems = %q, want problems starting %q", got, tc.want)
			}
		})
	}
}

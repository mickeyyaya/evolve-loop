//go:build acs

package repocontract

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet"
)

func TestRepoContract_GoldenParity(t *testing.T) {
	treeRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	files, _, err := rawgitratchet.BoundTestFiles(treeRoot)
	if err != nil {
		t.Fatal(err)
	}
	streamed := sharedTreeScan(t)
	whole := goldenWholeTreeScan(t, treeRoot, files)
	if whole.RootHelpers == 0 || len(whole.Reading) < 20 {
		t.Fatalf("the old scanner went blind on the real tree (root helpers %d, reading dirs %d), so parity proves nothing", whole.RootHelpers, len(whole.Reading))
	}
	for _, difference := range diffScans(whole, streamed) {
		t.Error(difference)
	}
}

func TestRepoContract_GoldenParityDetectsADivergence(t *testing.T) {
	base := treeScan{Writers: []string{"a: os.WriteFile"}, RootHelpers: 2, Reading: map[string][]string{"internal/x": {"TestA"}}}
	if got := diffScans(base, base); len(got) != 0 {
		t.Fatalf("equal scans must not differ: %v", got)
	}
	cases := []struct {
		name  string
		other treeScan
	}{
		{"a writer lost", treeScan{RootHelpers: 2, Reading: base.Reading}},
		{"a root helper lost", treeScan{Writers: base.Writers, RootHelpers: 1, Reading: base.Reading}},
		{"a reading test lost", treeScan{Writers: base.Writers, RootHelpers: 2, Reading: map[string][]string{"internal/x": {}}}},
		{"a reading directory lost", treeScan{Writers: base.Writers, RootHelpers: 2, Reading: map[string][]string{}}},
	}
	for _, tc := range cases {
		if len(diffScans(base, tc.other)) == 0 {
			t.Errorf("%s: the comparison reported no difference", tc.name)
		}
	}
}

func diffScans(want, got treeScan) []string {
	var out []string
	if w, g := slices.Sorted(slices.Values(want.Writers)), slices.Sorted(slices.Values(got.Writers)); !slices.Equal(w, g) {
		out = append(out, fmt.Sprintf("writers differ: old %v, new %v", w, g))
	}
	if want.RootHelpers != got.RootHelpers {
		out = append(out, fmt.Sprintf("root helpers differ: old %d, new %d", want.RootHelpers, got.RootHelpers))
	}
	for _, dir := range slices.Sorted(maps.Keys(want.Reading)) {
		tests, ok := got.Reading[dir]
		if !ok || !slices.Equal(want.Reading[dir], tests) {
			out = append(out, fmt.Sprintf("reading tests of %s differ: old %v, new %v", dir, want.Reading[dir], tests))
		}
	}
	for _, dir := range slices.Sorted(maps.Keys(got.Reading)) {
		if _, ok := want.Reading[dir]; !ok {
			out = append(out, fmt.Sprintf("%s reads the tree only in the new scan", dir))
		}
	}
	return out
}

func goldenWholeTreeScan(t *testing.T, treeRoot string, files []string) treeScan {
	t.Helper()
	fset := token.NewFileSet()
	byPackage := map[string][]*ast.File{}
	byDir := map[string][]*ast.File{}
	for _, rel := range files {
		file, err := parser.ParseFile(fset, filepath.Join(treeRoot, filepath.FromSlash(rel)), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		key := packageKey(path.Dir(rel), file.Name.Name)
		byPackage[key] = append(byPackage[key], file)
		if !strings.HasPrefix(rel, "acs/") {
			byDir[path.Dir(rel)] = append(byDir[path.Dir(rel)], file)
		}
	}
	whole := treeScan{Reading: map[string][]string{}}
	for _, key := range slices.Sorted(maps.Keys(byPackage)) {
		scan := newTreeWriteScan(byPackage[key])
		whole.RootHelpers += scan.rootHelpers()
		for _, finding := range scan.findings(fset) {
			whole.Writers = append(whole.Writers, relativeTo(treeRoot, finding))
		}
	}
	for dir, parsed := range byDir {
		if tests, reads := readingTestsOf(parsed, dir); reads {
			whole.Reading[dir] = tests
		}
	}
	return whole
}

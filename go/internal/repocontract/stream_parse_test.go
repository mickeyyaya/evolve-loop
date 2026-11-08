package repocontract

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet"
)

func writeTree(t *testing.T, sources map[string]string) (string, []string) {
	t.Helper()
	dir := t.TempDir()
	var rels []string
	for rel, src := range sources {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		rels = append(rels, rel)
	}
	slices.Sort(rels)
	return dir, rels
}

func ignoreUnit(treeUnit) error { return nil }

func waitReleased(flag *atomic.Bool, attempts int) bool {
	for attempt := 0; attempt < attempts; attempt++ {
		runtime.GC()
		if flag.Load() {
			return true
		}
		runtime.Gosched()
	}
	return flag.Load()
}

func TestStreamParse_EachBoundFileParsesOnceAcrossBothScanners(t *testing.T) {
	treeRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	files, _, err := rawgitratchet.BoundTestFiles(treeRoot)
	if err != nil {
		t.Fatal(err)
	}
	first := sharedTreeScan(t)
	second := sharedTreeScan(t)
	if first.RootHelpers == 0 || first.RootHelpers != second.RootHelpers {
		t.Fatalf("the shared scan must give one answer to both scanners: root helpers %d then %d", first.RootHelpers, second.RootHelpers)
	}
	counts := fileParseCounts()
	var wrong []string
	for _, rel := range files {
		key := filepath.Join(treeRoot, filepath.FromSlash(rel))
		if counts[key] != 1 {
			wrong = append(wrong, rel)
		}
	}
	if len(wrong) > 0 {
		t.Errorf("%d of %d bound test files did not parse exactly once in this test binary; first: %v", len(wrong), len(files), wrong[:min(len(wrong), 5)])
	}
}

func TestStreamParse_TheCounterCountsEveryParseOfAFile(t *testing.T) {
	dir, rels := writeTree(t, map[string]string{
		"a/a_test.go": "package a\n",
		"b/b_test.go": "package b\n",
	})
	for pass := 0; pass < 2; pass++ {
		if err := streamTestTree(dir, rels, ignoreUnit); err != nil {
			t.Fatal(err)
		}
	}
	counts := fileParseCounts()
	for _, rel := range rels {
		if got := counts[filepath.Join(dir, filepath.FromSlash(rel))]; got != 2 {
			t.Errorf("%s was streamed twice but the counter reads %d", rel, got)
		}
	}
}

func TestStreamParse_GroupsFilesByDirectoryAndKeepsFirstSeenOrder(t *testing.T) {
	dir, rels := writeTree(t, map[string]string{
		"p/a_test.go":     "package p\n",
		"p/sub/s_test.go": "package sub\n",
		"p/z_test.go":     "package p_test\n",
	})
	type seen struct {
		dir   string
		rels  []string
		names []string
		file0 string
	}
	var visits []seen
	err := streamTestTree(dir, rels, func(unit treeUnit) error {
		s := seen{dir: unit.Dir, rels: slices.Clone(unit.Rels)}
		for _, file := range unit.Files {
			s.names = append(s.names, file.Name.Name)
		}
		s.file0 = unit.Fset.Position(unit.Files[0].Pos()).Filename
		visits = append(visits, s)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(visits) != 2 {
		t.Fatalf("want 2 visits (p, then p/sub), got %d: %+v", len(visits), visits)
	}
	if visits[0].dir != "p" || !slices.Equal(visits[0].rels, []string{"p/a_test.go", "p/z_test.go"}) || !slices.Equal(visits[0].names, []string{"p", "p_test"}) {
		t.Errorf("the first visit must hold both files of p in the order given: %+v", visits[0])
	}
	if visits[1].dir != "p/sub" || !slices.Equal(visits[1].rels, []string{"p/sub/s_test.go"}) {
		t.Errorf("the second visit must hold p/sub: %+v", visits[1])
	}
	if want := filepath.Join(dir, "p", "a_test.go"); visits[0].file0 != want {
		t.Errorf("a position must name the file by its path under the root: got %q want %q", visits[0].file0, want)
	}
}

func TestStreamParse_ReleasesEachUnitBeforeTheNextOne(t *testing.T) {
	dir, rels := writeTree(t, map[string]string{
		"a/a_test.go": "package a\n",
		"b/b_test.go": "package b\n",
		"c/c_test.go": "package c\n",
	})
	var released [3]atomic.Bool
	index := 0
	err := streamTestTree(dir, rels, func(unit treeUnit) error {
		i := index
		index++
		runtime.SetFinalizer(unit.Files[0], func(*ast.File) { released[i].Store(true) })
		if i > 0 && !waitReleased(&released[i-1], 200) {
			t.Errorf("the ASTs of unit %d were still reachable when unit %d was visited", i-1, i)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if index != 3 {
		t.Fatalf("want 3 visits, got %d", index)
	}
	if !waitReleased(&released[2], 200) {
		t.Error("the ASTs of the last unit were still reachable after the walk returned")
	}
}

func TestStreamParse_ARetainingVisitorIsSeenToHoldItsUnits(t *testing.T) {
	dir, rels := writeTree(t, map[string]string{
		"a/a_test.go": "package a\n",
		"b/b_test.go": "package b\n",
	})
	var released atomic.Bool
	var held []treeUnit
	err := streamTestTree(dir, rels, func(unit treeUnit) error {
		if len(held) == 0 {
			runtime.SetFinalizer(unit.Files[0], func(*ast.File) { released.Store(true) })
		}
		held = append(held, unit)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if waitReleased(&released, 20) {
		t.Error("the release probe reports an AST as released while a visitor still holds it; it proves nothing")
	}
	if len(held) != 2 {
		t.Errorf("want 2 held units, got %d", len(held))
	}
}

func TestStreamParse_AParseErrorNamesTheFileAndStopsTheWalk(t *testing.T) {
	dir, rels := writeTree(t, map[string]string{
		"a/a_test.go":     "package a\n",
		"bad/bad_test.go": "package bad\nfunc (\n",
		"z/z_test.go":     "package z\n",
	})
	var visited []string
	err := streamTestTree(dir, rels, func(unit treeUnit) error {
		visited = append(visited, unit.Dir)
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "bad_test.go") {
		t.Fatalf("a parse error must name the file, got %v", err)
	}
	if slices.Contains(visited, "bad") || slices.Contains(visited, "z") {
		t.Errorf("the walk must stop at the bad directory, visited %v", visited)
	}
}

func TestStreamParse_AVisitorErrorStopsTheWalk(t *testing.T) {
	dir, rels := writeTree(t, map[string]string{
		"a/a_test.go": "package a\n",
		"b/b_test.go": "package b\n",
	})
	stop := errors.New("stop here")
	visits := 0
	err := streamTestTree(dir, rels, func(unit treeUnit) error {
		visits++
		return stop
	})
	if !errors.Is(err, stop) || visits != 1 {
		t.Errorf("the walk must return the visitor error after one visit, got err=%v visits=%d", err, visits)
	}
}

func TestStreamParse_TheTwoScannerTestsUseTheSharedScanAndParseNothingThemselves(t *testing.T) {
	for _, name := range []string{"realtree_writes_test.go", "packages_test.go"} {
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		parsesOwn, readsShared := false, false
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fun := call.Fun.(type) {
			case *ast.SelectorExpr:
				if pkg, ok := fun.X.(*ast.Ident); ok && pkg.Name == "parser" {
					parsesOwn = true
				}
			case *ast.Ident:
				if fun.Name == "sharedTreeScan" {
					readsShared = true
				}
			}
			return true
		})
		if parsesOwn {
			t.Errorf("%s calls the parser itself; the parse belongs to the shared scan", name)
		}
		if !readsShared {
			t.Errorf("%s does not call sharedTreeScan", name)
		}
	}
}

func TestStreamParse_ScanFlagsAWriteAndNamesTheReadingTest(t *testing.T) {
	const planting = `package x

import (
	"os"
	"path/filepath"
	"testing"
)

func repoRoot() string { return filepath.Join("..", "..") }

func TestPlant(t *testing.T) {
	os.WriteFile(filepath.Join(repoRoot(), "planted"), nil, 0o644)
}
`
	const readingOnly = `package x_test

import (
	"path/filepath"
	"testing"
)

func TestReadsToo(t *testing.T) {
	_, _ = filepath.Abs("../..")
}
`
	dir, rels := writeTree(t, map[string]string{
		"internal/x/x_test.go":     planting,
		"internal/x/y_test.go":     readingOnly,
		"acs/cycle1/p_test.go":     strings.Replace(planting, "package x", "package cycle1", 1),
		"internal/clean/c_test.go": "package clean\n\nfunc TestNothing() {}\n",
	})
	scan, err := scanTestTree(dir, rels)
	if err != nil {
		t.Fatal(err)
	}
	var planted []string
	for _, finding := range scan.Writers {
		planted = append(planted, finding[:strings.Index(finding, ":")])
	}
	slices.Sort(planted)
	if want := []string{"acs/cycle1/p_test.go", "internal/x/x_test.go"}; !slices.Equal(planted, want) {
		t.Errorf("writers = %v, want files %v (every finding names its file under the root)", scan.Writers, want)
	}
	if scan.RootHelpers != 2 {
		t.Errorf("root helpers = %d, want 2", scan.RootHelpers)
	}
	if got := scan.Reading["internal/x"]; !slices.Equal(got, []string{"TestPlant", "TestReadsToo"}) {
		t.Errorf("reading[internal/x] = %v, want both packages of the directory merged", got)
	}
	if _, ok := scan.Reading["acs/cycle1"]; ok {
		t.Error("an acs directory must never join the reading map")
	}
	if _, ok := scan.Reading["internal/clean"]; ok {
		t.Error("a directory that does not read the tree must not join the reading map")
	}
}

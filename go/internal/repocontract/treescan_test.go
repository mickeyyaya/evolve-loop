package repocontract

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet"
)

type treeUnit struct {
	Dir   string
	Fset  *token.FileSet
	Rels  []string
	Files []*ast.File
}

type treeScan struct {
	Writers       []string
	RootHelpers   int
	Reading       map[string][]string
	PeakHeapBytes uint64
	HeapSamples   int
}

var (
	parseMu     sync.Mutex
	parsedCount = map[string]int{}
)

func fileParseCounts() map[string]int {
	parseMu.Lock()
	defer parseMu.Unlock()
	return maps.Clone(parsedCount)
}

func streamTestTree(root string, rels []string, visit func(treeUnit) error) error {
	var dirs []string
	byDir := map[string][]string{}
	for _, rel := range rels {
		d := path.Dir(rel)
		if _, ok := byDir[d]; !ok {
			dirs = append(dirs, d)
		}
		byDir[d] = append(byDir[d], rel)
	}
	for _, d := range dirs {
		if err := streamOne(root, d, byDir[d], visit); err != nil {
			return err
		}
	}
	return nil
}

func streamOne(root, d string, rels []string, visit func(treeUnit) error) error {
	fset := token.NewFileSet()
	files := make([]*ast.File, 0, len(rels))
	for _, rel := range rels {
		full := filepath.Join(root, filepath.FromSlash(rel))
		parseMu.Lock()
		parsedCount[full]++
		parseMu.Unlock()
		f, err := parser.ParseFile(fset, full, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		files = append(files, f)
	}
	return visit(treeUnit{Dir: d, Fset: fset, Rels: rels, Files: files})
}

func scanTestTree(root string, rels []string) (treeScan, error) {
	out := treeScan{Reading: map[string][]string{}}
	runtime.GC()
	sample := func() {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		out.HeapSamples++
		out.PeakHeapBytes = max(out.PeakHeapBytes, m.HeapAlloc)
	}
	err := streamTestTree(root, rels, func(u treeUnit) error {
		sample()
		byPkg := map[string][]*ast.File{}
		for _, f := range u.Files {
			byPkg[f.Name.Name] = append(byPkg[f.Name.Name], f)
		}
		for _, name := range slices.Sorted(maps.Keys(byPkg)) {
			s := newTreeWriteScan(byPkg[name])
			out.RootHelpers += s.rootHelpers()
			for _, fd := range s.findings(u.Fset) {
				out.Writers = append(out.Writers, relativeTo(root, fd))
			}
		}
		var read []*ast.File
		for i, f := range u.Files {
			if !strings.HasPrefix(u.Rels[i], "acs/") {
				read = append(read, f)
			}
		}
		if len(read) > 0 {
			if tests, reads := readingTestsOf(read, u.Dir); reads {
				out.Reading[u.Dir] = tests
			}
		}
		return nil
	})
	return out, err
}

var (
	sharedOnce sync.Once
	sharedVal  treeScan
	sharedErr  error
)

func sharedTreeScan(t *testing.T) treeScan {
	t.Helper()
	sharedOnce.Do(func() {
		root, err := filepath.Abs("../..")
		if err != nil {
			sharedErr = err
			return
		}
		files, _, err := rawgitratchet.BoundTestFiles(root)
		if err != nil {
			sharedErr = err
			return
		}
		sharedVal, sharedErr = scanTestTree(root, files)
	})
	if sharedErr != nil {
		t.Fatal(sharedErr)
	}
	return sharedVal
}

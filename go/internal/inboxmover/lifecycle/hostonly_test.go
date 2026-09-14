package lifecycle

// hostonly_test.go — ADR-0079 D3 preserved by a pin (§6 test 44): Mover.Release
// is exported here (the compiler cannot hide it from the host), so the ONE
// public door into the cycle-outcome lifecycle — inboxmover.ApplyCycleOutcome —
// stays the only door by keeping internal/inboxmover the leaf's only importer.

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const leafImportPath = "github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"

func TestLifecycle_OnlyHostImportsTheLeaf(t *testing.T) {
	moduleRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var importers []string
	walk := func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if n := entry.Name(); n == "vendor" || n == "bin" || n == "testdata" || (strings.HasPrefix(n, ".") && path != moduleRoot) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return perr
		}
		for _, imp := range f.Imports {
			if strings.Trim(imp.Path.Value, `"`) == leafImportPath {
				rel, _ := filepath.Rel(moduleRoot, path)
				importers = append(importers, filepath.ToSlash(filepath.Dir(rel)))
			}
		}
		return nil
	}
	if err := filepath.WalkDir(moduleRoot, walk); err != nil {
		t.Fatal(err)
	}
	for _, dir := range importers {
		if dir != "internal/inboxmover" {
			t.Errorf("%s imports the leaf directly — every caller goes through the internal/inboxmover facades (ADR-0079 D3: ApplyCycleOutcome is the one public door)", dir)
		}
	}
	if len(importers) == 0 {
		t.Error("the host must import the leaf (the facades are the seam)")
	}
}

package signalcenter

// importgraph_test.go — the package is a leaf (design §6): standard library plus
// internal/log, nothing else under internal/, so core, bridge and cmd can all
// depend on it without an import cycle.

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

const internalLog = "github.com/mickeyyaya/evolve-loop/go/internal/log"

func TestImportGraph_LeafPackageImportsOnlyInternalLog(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			switch {
			case path == internalLog:
			case strings.Contains(path, "/internal/"):
				t.Errorf("%s imports %s: signalcenter must stay a leaf (stdlib + internal/log only)", name, path)
			case strings.Contains(strings.SplitN(path, "/", 2)[0], "."):
				t.Errorf("%s imports third-party %s: stdlib only", name, path)
			}
		}
	}
}

package observerengine

// importgraph_test.go — the package is a leaf (ADR-0103 unit 12 §2): stdlib
// plus internal/recovery (the StallPolicy strategy) and internal/signalcenter
// (the module's producer) — never core, adapters, bridge or policy (the
// failurelearning/importgraph_test.go idiom).

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

var allowedImports = map[string]bool{
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery":     true,
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter": true,
}

func TestImportGraph_LeafImportsOnlyTheDeclaredPackages(t *testing.T) {
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
			case allowedImports[path]:
			case strings.Contains(path, "/internal/"):
				t.Errorf("%s imports %s: the engine is a leaf — stdlib plus recovery and signalcenter only", name, path)
			case strings.Contains(strings.SplitN(path, "/", 2)[0], "."):
				t.Errorf("%s imports third-party %s: stdlib only", name, path)
			}
		}
	}
}

package subagentrun

// importgraph_test.go — the package is a leaf under internal/subagent
// (ADR-0103 unit 16 §2): stdlib plus the three named internal packages, never
// internal/subagent, internal/core, internal/bridge, capability or resolvellm
// (the compiler is the cycle guard; this is the leaf-ness declaration —
// signalcenter/importgraph_test.go idiom).

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

var allowedImports = map[string]bool{
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate":   true,
	"github.com/mickeyyaya/evolve-loop/go/internal/detectcli":    true,
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
				t.Errorf("%s imports %s: the dispatcher is a leaf — stdlib plus the three declared packages only", name, path)
			case strings.Contains(strings.SplitN(path, "/", 2)[0], "."):
				t.Errorf("%s imports third-party %s: stdlib only", name, path)
			}
		}
	}
}

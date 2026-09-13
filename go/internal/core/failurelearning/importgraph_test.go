package failurelearning

// importgraph_test.go — the package is a leaf under core (ADR-0103 unit 03b
// §2): stdlib plus the nine named internal packages, never internal/core
// itself (the compiler is the cycle guard; this is the leaf-ness declaration —
// signalcenter/importgraph_test.go idiom).

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

var allowedImports = map[string]bool{
	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover": true,
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate":     true,
	"github.com/mickeyyaya/evolve-loop/go/internal/faillearn":      true,
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog":     true,
	"github.com/mickeyyaya/evolve-loop/go/internal/paths":          true,
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract":  true,
	"github.com/mickeyyaya/evolve-loop/go/internal/policy":         true,
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence":     true,
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter":   true,
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
				t.Errorf("%s imports %s: the engine is a leaf — stdlib plus the nine declared packages only", name, path)
			case strings.Contains(strings.SplitN(path, "/", 2)[0], "."):
				t.Errorf("%s imports third-party %s: stdlib only", name, path)
			}
		}
	}
}

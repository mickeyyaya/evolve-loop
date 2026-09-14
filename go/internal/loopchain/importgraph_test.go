package loopchain

// importgraph_test.go — the package is a core-free leaf beside cmd/evolve
// (ADR-0103 unit 13 §2): stdlib plus the six named internal packages — never
// internal/core, internal/fleet or pkg/version (the running commit and the
// quota-pause reader are injected) — signalcenter/importgraph_test.go idiom.

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

var allowedImports = map[string]bool{
	"github.com/mickeyyaya/evolve-loop/go/internal/gc":             true,
	"github.com/mickeyyaya/evolve-loop/go/internal/paths":          true,
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity": true,
	"github.com/mickeyyaya/evolve-loop/go/internal/policy":         true,
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease":       true,
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
			case strings.Contains(path, "/internal/") || strings.Contains(path, "/pkg/"):
				t.Errorf("%s imports %s: the chain engine is a core-free leaf — stdlib plus the six declared packages only", name, path)
			case strings.Contains(strings.SplitN(path, "/", 2)[0], "."):
				t.Errorf("%s imports third-party %s: stdlib only", name, path)
			}
		}
	}
}

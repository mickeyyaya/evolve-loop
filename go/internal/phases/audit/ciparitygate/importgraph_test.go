package ciparitygate

// importgraph_test.go — the package is a leaf under the audit phase (ADR-0103
// unit 14 §2): stdlib plus the eight named internal packages, never
// internal/core, internal/changedpkgs or the host package (the compiler is the
// cycle guard; this is the leaf-ness declaration — signalcenter/importgraph_test.go
// idiom).

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

var allowedImports = map[string]bool{
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock": true,
	"github.com/mickeyyaya/evolve-loop/go/internal/apicover":       true,
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity":       true,
	"github.com/mickeyyaya/evolve-loop/go/internal/codequality":    true,
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate":     true,
	"github.com/mickeyyaya/evolve-loop/go/internal/paths":          true,
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter":   true,
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec":        true,
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
				t.Errorf("%s imports %s: the gates are a leaf — stdlib plus the eight declared packages only", name, path)
			case strings.Contains(strings.SplitN(path, "/", 2)[0], "."):
				t.Errorf("%s imports third-party %s: stdlib only", name, path)
			}
		}
	}
}

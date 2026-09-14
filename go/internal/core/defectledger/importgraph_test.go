package defectledger

// importgraph_test.go — the package is a leaf under core (ADR-0103 unit 09
// §2): stdlib plus the five named internal packages, never internal/core,
// carryover or phasecontract (the compiler is the cycle guard; this is the
// leaf-ness declaration — signalcenter/importgraph_test.go idiom). The leaf
// also never writes stderr: its failure modes are codes, not prose lines.

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

var allowedImports = map[string]bool{
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite":  true,
	"github.com/mickeyyaya/evolve-loop/go/internal/continuation": true,
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate":   true,
	"github.com/mickeyyaya/evolve-loop/go/internal/paths":        true,
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter": true,
}

func productionSources(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, entry := range entries {
		if name := entry.Name(); strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			out = append(out, name)
		}
	}
	return out
}

func TestImportGraph_LeafImportsOnlyTheDeclaredPackages(t *testing.T) {
	fset := token.NewFileSet()
	for _, name := range productionSources(t) {
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			switch {
			case allowedImports[path]:
			case strings.Contains(path, "/internal/"):
				t.Errorf("%s imports %s: the ledger is a leaf — stdlib plus the five declared packages only", name, path)
			case strings.Contains(strings.SplitN(path, "/", 2)[0], "."):
				t.Errorf("%s imports third-party %s: stdlib only", name, path)
			}
		}
	}
}

func TestLeaf_NeverWritesStderr(t *testing.T) {
	for _, name := range productionSources(t) {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "os.Stderr") || strings.Contains(string(body), "os.Stdout") {
			t.Errorf("%s writes to stderr: the unit reports through the Signal Center only", name)
		}
	}
}

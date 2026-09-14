package advisor

// importgraph_test.go — the package is a leaf under core (ADR-0103 unit 04
// §2): stdlib plus the fourteen named internal packages, never internal/core
// itself, clihealth, gitexec or the carryover lifecycle (the compiler is the
// cycle guard; this is the leaf-ness declaration — signalcenter/
// importgraph_test.go idiom).

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

var allowedImports = map[string]bool{
	"github.com/mickeyyaya/evolve-loop/go/internal/config":        true,
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate":    true,
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute":      true,
	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog":  true,
	"github.com/mickeyyaya/evolve-loop/go/internal/panetrust":     true,
	"github.com/mickeyyaya/evolve-loop/go/internal/paths":         true,
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig":   true,
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract": true,
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec":     true,
	"github.com/mickeyyaya/evolve-loop/go/internal/policy":        true,
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles":      true,
	"github.com/mickeyyaya/evolve-loop/go/internal/router":        true,
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter":  true,
	"github.com/mickeyyaya/evolve-loop/go/internal/textcap":       true,
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
				t.Errorf("%s imports %s: the advisor is a leaf — stdlib plus the fourteen declared packages only", name, path)
			case strings.Contains(strings.SplitN(path, "/", 2)[0], "."):
				t.Errorf("%s imports third-party %s: stdlib only", name, path)
			}
		}
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "os.Stderr") || strings.Contains(string(body), "advisor.New(") {
			t.Errorf("%s: the leaf never writes stderr and never spells its own construction", name)
		}
	}
}

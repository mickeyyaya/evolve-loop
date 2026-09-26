package loopwave

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

var allowedImports = map[string]bool{
	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory": true,
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth":     true,
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet":         true,
	"github.com/mickeyyaya/evolve-loop/go/internal/fleetbudget":   true,
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch":    true,
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover":    true,
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv":        true,
	"github.com/mickeyyaya/evolve-loop/go/internal/paths":         true,
	"github.com/mickeyyaya/evolve-loop/go/internal/policy":        true,
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate":    true,
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter":  true,
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap":     true,
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
				t.Errorf("%s imports %s: the engine is a leaf — stdlib plus the twelve declared packages only", name, path)
			case strings.Contains(strings.SplitN(path, "/", 2)[0], "."):
				t.Errorf("%s imports third-party %s: stdlib only", name, path)
			}
		}
	}
}

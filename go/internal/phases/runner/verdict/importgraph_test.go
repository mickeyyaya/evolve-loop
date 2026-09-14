package verdict

// importgraph_test.go — the package is a leaf under the runner (ADR-0103 unit
// 11 §2): stdlib plus the six named internal packages, never the host, never
// internal/log, the bridge, the env or the process streams (the compiler is
// the cycle guard; this is the leaf-ness declaration — the
// signalcenter/importgraph_test.go idiom). Direct imports only: coherence
// reaches policy transitively, which is fine.

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var allowedImports = map[string]bool{
	"github.com/mickeyyaya/evolve-loop/go/internal/coherence":     true,
	"github.com/mickeyyaya/evolve-loop/go/internal/core":          true,
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable":   true,
	"github.com/mickeyyaya/evolve-loop/go/internal/paths":         true,
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract": true,
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter":  true,
}

func nonTestSources(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			out = append(out, name)
		}
	}
	return out
}

func TestImportGraph_LeafImportsOnlyTheDeclaredPackages(t *testing.T) {
	fset := token.NewFileSet()
	for _, name := range nonTestSources(t) {
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			switch {
			case allowedImports[path]:
			case strings.Contains(path, "/internal/"):
				t.Errorf("%s imports %s: the engine is a leaf — stdlib plus the six declared packages only", name, path)
			case strings.Contains(strings.SplitN(path, "/", 2)[0], "."):
				t.Errorf("%s imports third-party %s: stdlib only", name, path)
			}
		}
	}
}

// Test 13c — the leaf writes no stderr and reads no env: its only voice is the
// Signal Center. A source scan, because a stray fmt.Fprintf(os.Stderr, …) is
// invisible to every other test.
func TestNoStderrNoEnv_InTheLeaf(t *testing.T) {
	for _, name := range nonTestSources(t) {
		src, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatal(err)
		}
		for _, needle := range []string{"os.Stderr", "os.Stdout", "os.Getenv", "os.LookupEnv", "os.Environ", "log.Diag", "fmt.Print", "fmt.Fprint"} {
			if strings.Contains(string(src), needle) {
				t.Errorf("%s uses %s — the leaf reports through the Center only", name, needle)
			}
		}
	}
}

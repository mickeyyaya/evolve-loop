package bridge

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

var phaseVerifierPackages = []string{
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable",
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/phasecmd",
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner/verdict",
}

func TestBridge_ReportsEvidenceButNeverImportsAPhaseVerifier(t *testing.T) {
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
			for _, verifier := range phaseVerifierPackages {
				if path == verifier || strings.HasPrefix(path, verifier+"/") {
					t.Errorf("%s imports %s: the bridge reports completion evidence, and core runs phase verify on it", name, path)
				}
			}
		}
	}
}

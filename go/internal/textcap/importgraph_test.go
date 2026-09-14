package textcap

// importgraph_test.go — textcap is a stdlib-only leaf (ADR-0103 unit 04 §2):
// the two rune-cap rules must never grow a dependency, or the advisor and the
// carryover lifecycle would inherit it (signalcenter/importgraph_test.go idiom).

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestImportGraph_StdlibOnly(t *testing.T) {
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
			if strings.Contains(strings.SplitN(path, "/", 2)[0], ".") {
				t.Errorf("%s imports %s: textcap is stdlib only", name, path)
			}
		}
	}
}

package lanetime

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestImportGraph_TheReportImportsOnlyTheStandardLibrary(t *testing.T) {
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
			if path := strings.Trim(imp.Path.Value, `"`); strings.Contains(strings.SplitN(path, "/", 2)[0], ".") {
				t.Errorf("%s imports %s: the report is a leaf and must not import internal/core or any module package", name, path)
			}
		}
	}
}

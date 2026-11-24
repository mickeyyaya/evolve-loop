package modelquery

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackage_NeverImportsTheBridge(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no source file found (err %v)", err)
	}
	for _, name := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range parsed.Imports {
			if strings.Contains(imp.Path.Value, "/internal/bridge") {
				t.Errorf("%s imports %s; the bridge imports modelquery, so the reverse is a cycle", name, imp.Path.Value)
			}
		}
	}
}

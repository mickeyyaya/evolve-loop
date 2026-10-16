package recovery

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestStripFile_ImportsOnlyStrings(t *testing.T) {
	t.Parallel()
	file, err := parser.ParseFile(token.NewFileSet(), "strip.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse strip.go: %v", err)
	}
	var got []string
	for _, imp := range file.Imports {
		got = append(got, imp.Path.Value)
	}
	if len(got) != 1 || got[0] != `"strings"` {
		t.Fatalf("strip.go imports = %v, want only \"strings\": go/acs/cycle1123 compiles overlay mutants of StripAgentContent, and an import used only by it breaks them", got)
	}
}

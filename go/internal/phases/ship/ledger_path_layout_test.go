package ship

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestShipLedgerPathsComeFromThePathsLayout(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"audit.go", "carry.go"} {
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && strings.Contains(lit.Value, "ledger.jsonl") {
				t.Errorf("%s hard-codes %s; use paths.LedgerFileOf", file, lit.Value)
			}
			return true
		})
	}
}

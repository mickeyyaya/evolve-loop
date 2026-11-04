package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestTerminalPhaseLiteralHasOneHome(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") || name == "phase_terminal.go" {
			continue
		}
		f, perr := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if perr != nil {
			t.Fatal(perr)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if ok && lit.Kind == token.STRING {
				if v, _ := strconv.Unquote(lit.Value); v == string(PhaseAborted) {
					t.Errorf("%s: bare %q literal; use PhaseAborted or IsTerminalPhase", name, v)
				}
			}
			return true
		})
	}
}

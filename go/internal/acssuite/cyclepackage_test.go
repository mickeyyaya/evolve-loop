package acssuite

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCyclePackage_IsTheOneSpelling(t *testing.T) {
	t.Parallel()
	if got := CyclePackage(1605); got != "./acs/cycle1605" {
		t.Fatalf("CyclePackage(1605) = %q", got)
	}
	if got := currentCycleGoPkgDir("/m", 7); got != filepath.Join("/m", "acs", "cycle7") {
		t.Fatalf("currentCycleGoPkgDir must derive from CyclePackage: %q", got)
	}
}

func TestCyclePackage_NoOtherCodeInThePackageSpellsACyclePattern(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		for _, spelling := range cyclePatternSpellingsOutsideCyclePackage(t, name) {
			t.Errorf("%s spells a cycle predicate pattern (%s) outside CyclePackage; derive it from CyclePackage so the writer and its readers cannot drift", name, spelling)
		}
	}
}

func cyclePatternSpellingsOutsideCyclePackage(t *testing.T, name string) []string {
	t.Helper()
	src, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), name, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "CyclePackage" {
			continue
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if value, err := strconv.Unquote(lit.Value); err == nil && strings.HasPrefix(value, "./acs/cycle") {
				found = append(found, lit.Value)
			}
			return true
		})
	}
	return found
}

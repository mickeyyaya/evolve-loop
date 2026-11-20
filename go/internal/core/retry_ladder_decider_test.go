package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const retryladderImport = "github.com/mickeyyaya/evolve-loop/go/internal/retryladder"

func ladderImportName(file *ast.File) string {
	for _, imp := range file.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		if p != retryladderImport {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return path.Base(p)
	}
	return ""
}

func unwrapType(expr ast.Expr) ast.Expr {
	switch v := expr.(type) {
	case *ast.ArrayType:
		return unwrapType(v.Elt)
	case *ast.MapType:
		return unwrapType(v.Value)
	case *ast.StarExpr:
		return unwrapType(v.X)
	}
	return expr
}

func decisionTypeName(expr ast.Expr, pkg string) string {
	sel, ok := unwrapType(expr).(*ast.SelectorExpr)
	if !ok || (sel.Sel.Name != "Decision" && sel.Sel.Name != "Verdict") {
		return ""
	}
	if id, ok := sel.X.(*ast.Ident); ok && id.Name == pkg {
		return sel.Sel.Name
	}
	return ""
}

func ladderCall(expr ast.Expr, pkg string) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg
}

func namedFields(list *ast.FieldList, pkg string, into map[string]bool) bool {
	if list == nil {
		return false
	}
	named := false
	for _, f := range list.List {
		if decisionTypeName(f.Type, pkg) == "" {
			continue
		}
		for _, id := range f.Names {
			into[id.Name] = true
			named = true
		}
	}
	return named
}

func ladderValues(file *ast.File, pkg string) (results, tracked map[string]bool) {
	results, tracked = map[string]bool{}, map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.FuncType:
			namedFields(v.Params, pkg, tracked)
			namedFields(v.Results, pkg, results)
		case *ast.ValueSpec:
			if decisionTypeName(v.Type, pkg) != "" {
				for _, id := range v.Names {
					tracked[id.Name] = true
				}
			}
		case *ast.AssignStmt:
			if len(v.Rhs) == 1 && ladderCall(v.Rhs[0], pkg) {
				for _, lhs := range v.Lhs {
					if id, ok := lhs.(*ast.Ident); ok {
						tracked[id.Name] = true
					}
				}
			}
		}
		return true
	})
	return results, tracked
}

func fieldWrites(file *ast.File, names map[string]bool) []string {
	var found []string
	ast.Inspect(file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, lhs := range assign.Lhs {
			if sel, ok := lhs.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && names[id.Name] {
					found = append(found, "field write "+id.Name+"."+sel.Sel.Name)
				}
			}
		}
		return true
	})
	return found
}

func decisionBuilds(file *ast.File, pkg string) []string {
	results, tracked := ladderValues(file, pkg)
	found := fieldWrites(file, mergeNames(results, tracked))
	for name := range results {
		found = append(found, "named result "+name)
	}
	ast.Inspect(file, func(n ast.Node) bool {
		var typ ast.Expr
		switch v := n.(type) {
		case *ast.CompositeLit:
			typ = v.Type
		case *ast.ValueSpec:
			typ = v.Type
		case *ast.CallExpr:
			if id, ok := v.Fun.(*ast.Ident); ok && id.Name == "new" && len(v.Args) == 1 {
				typ = v.Args[0]
			}
		}
		if name := decisionTypeName(typ, pkg); name != "" {
			found = append(found, name)
		}
		return true
	})
	return found
}

func mergeNames(a, b map[string]bool) map[string]bool {
	out := map[string]bool{}
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}

func decisionBuildsIn(t *testing.T, root string) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		if strings.Contains(filepath.ToSlash(p), "/internal/retryladder/") {
			return nil
		}
		file, perr := parser.ParseFile(token.NewFileSet(), p, nil, 0)
		if perr != nil {
			return perr
		}
		pkg := ladderImportName(file)
		if pkg == "" {
			return nil
		}
		for _, name := range decisionBuilds(file, pkg) {
			found = append(found, p+": "+name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func TestNoDecisionLiteralOutsideDecider(t *testing.T) {
	t.Parallel()
	if found := decisionBuildsIn(t, "../.."); len(found) > 0 {
		t.Fatalf("a retry ladder decision or verdict built outside retryladder: %v", found)
	}
}

func parseDecisionTestSnippet(t *testing.T, name, src string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func TestDecisionGuardSeesEveryShapeAndAlias(t *testing.T) {
	t.Parallel()
	src := `package x
import ladder "` + retryladderImport + `"
func f() { _ = ladder.Verdict{}; var d ladder.Decision; _ = new(ladder.Verdict); _ = d }
`
	file := parseDecisionTestSnippet(t, "x.go", src)
	pkg := ladderImportName(file)
	if pkg != "ladder" {
		t.Fatalf("alias not resolved: %q", pkg)
	}
	if got := decisionBuilds(file, pkg); len(got) != 3 {
		t.Fatalf("literal, var and new must each be seen, got %v", got)
	}
	for _, shape := range []string{
		"func a() (d ladder.Decision) { d.Action = ladder.ActionRaise; return }",
		"func a(in ladder.Input) { v := ladder.Classify(in); v.Class = ladder.Capability }",
		"var _ = []ladder.Decision{{Action: ladder.ActionStop}}",
		"var _ = map[string]*ladder.Verdict{\"k\": {}}",
	} {
		f := parseDecisionTestSnippet(t, "s.go", "package s\nimport ladder \""+retryladderImport+"\"\n"+shape+"\n")
		if len(decisionBuilds(f, "ladder")) == 0 {
			t.Errorf("shape escaped the guard: %s", shape)
		}
	}
	plain := parseDecisionTestSnippet(t, "y.go", "package y\nimport \""+retryladderImport+"\"\nvar _ retryladder.Class\n")
	if ladderImportName(plain) != "retryladder" || len(decisionBuilds(plain, "retryladder")) != 0 {
		t.Fatal("an unaliased import with no decision type must resolve and report nothing")
	}
	none := parseDecisionTestSnippet(t, "z.go", "package z\n")
	if ladderImportName(none) != "" {
		t.Fatal("no import, no name")
	}
}

func TestGuardCoversTheModuleRootNotOnlyInternal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	body := "package m\nimport \"" + retryladderImport + "\"\nvar _ = retryladder.Verdict{}\n"
	writeFile(t, filepath.Join(dir, "cmd", "m.go"), body)
	if got := decisionBuildsIn(t, dir); len(got) != 1 {
		t.Fatalf("a build under cmd must be found, got %v", got)
	}
}

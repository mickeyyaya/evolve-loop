package sysexec

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/structure"
)

var pipelineSourceDirs = []string{"cmd", "internal", "pkg"}

const sysexecDir = "internal/sysexec"

func TestPipelineCode_StartsEveryProcessThroughCommand(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	files, err := structure.SourceFiles(root, pipelineSourceDirs...)
	if err != nil {
		t.Fatalf("walk %v: %v", pipelineSourceDirs, err)
	}
	var offenders []string
	for _, rel := range files {
		if filepath.Dir(rel) != sysexecDir {
			offenders = append(offenders, directExecCalls(t, filepath.Join(root, rel), rel)...)
		}
	}
	if len(offenders) > 0 {
		t.Errorf("%d call(s) start a process without sysexec.Command, so they have no Cancel and no WaitDelay:\n%s", len(offenders), strings.Join(offenders, "\n"))
	}
}

func TestDirectExecCalls_FindsAliasedAndPlainCalls(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "x.go")
	src := "package x\nimport (\n\tosexec \"os/exec\"\n\t\"context\"\n)\nfunc f() { _ = osexec.Command(\"a\"); _ = osexec.CommandContext(context.Background(), \"b\"); _ = &osexec.Cmd{Path: \"c\"} }\n"
	if err := writeFile(path, src); err != nil {
		t.Fatal(err)
	}

	got := directExecCalls(t, path, "x.go")

	if len(got) != 3 {
		t.Errorf("found %q, want the Command call, the CommandContext call and the Cmd literal", got)
	}
}

func directExecCalls(t *testing.T, path, rel string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	name := execImportName(file)
	if name == "" {
		return nil
	}
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		if sel := directExecUse(n, name); sel != "" {
			out = append(out, rel+":"+strconv.Itoa(fset.Position(n.Pos()).Line)+": "+name+"."+sel)
		}
		return true
	})
	return out
}

func execImportName(file *ast.File) string {
	for _, imp := range file.Imports {
		if imp.Path.Value != `"os/exec"` {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return "exec"
	}
	return ""
}

func directExecUse(n ast.Node, pkg string) string {
	switch x := n.(type) {
	case *ast.CallExpr:
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok && isPkg(sel.X, pkg) && (sel.Sel.Name == "Command" || sel.Sel.Name == "CommandContext") {
			return sel.Sel.Name
		}
	case *ast.CompositeLit:
		if sel, ok := x.Type.(*ast.SelectorExpr); ok && isPkg(sel.X, pkg) && sel.Sel.Name == "Cmd" {
			return "Cmd{}"
		}
	}
	return ""
}

func isPkg(e ast.Expr, pkg string) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == pkg
}

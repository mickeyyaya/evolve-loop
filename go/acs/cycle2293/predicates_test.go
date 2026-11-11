//go:build acs

package cycle2293

import (
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	gitexecImport    = "github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	remoteMainValue  = "origin/main"
	coreResumeBase   = "internal/core/resume_base.go"
	inboxmoverCurate = "internal/inboxmover/curate.go"
)

func moduleDir(t *testing.T) string {
	t.Helper()
	dir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatalf("no go module under the repo root: %v", err)
	}
	return dir
}

func productionGoFiles(t *testing.T, mod string) []string {
	t.Helper()
	files, err := acsassert.WalkModule(mod, func(rel string) bool {
		return strings.HasSuffix(rel, ".go") &&
			!strings.HasSuffix(rel, "_test.go") &&
			!strings.HasPrefix(rel, "acs/") &&
			!strings.HasPrefix(rel, "internal/gitexec/")
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

func parse(t *testing.T, path string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return f
}

func constantDeclsOf(f *ast.File, value string) []string {
	var hits []string
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || (gen.Tok != token.CONST && gen.Tok != token.VAR) {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, v := range vs.Values {
				lit, ok := v.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				if got, err := strconv.Unquote(lit.Value); err == nil && got == value {
					hits = append(hits, vs.Names[i].Name)
				}
			}
		}
	}
	return hits
}

func gitexecConst(t *testing.T, mod, name string) string {
	t.Helper()
	dir := filepath.Join(mod, "internal", "gitexec")
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, ok := pkgs["gitexec"]
	if !ok {
		t.Fatalf("no package gitexec in %s", dir)
	}
	var files []*ast.File
	for _, f := range pkg.Files {
		files = append(files, f)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil), Error: func(error) {}}
	checked, _ := conf.Check(gitexecImport, fset, files, nil)
	obj := checked.Scope().Lookup(name)
	c, ok := obj.(*types.Const)
	if !ok || !obj.Exported() {
		t.Fatalf("RED: gitexec exports no constant %s", name)
	}
	if c.Val().Kind() != constant.String {
		t.Fatalf("RED: gitexec.%s is not a string constant", name)
	}
	return constant.StringVal(c.Val())
}

func TestC2293_001_GitexecExportsTheRemoteAndMainRefConstants(t *testing.T) {
	mod := moduleDir(t)
	want := map[string]string{"Remote": "origin", "MainBranch": "main", "RemoteMain": remoteMainValue}
	for name, value := range want {
		if got := gitexecConst(t, mod, name); got != value {
			t.Errorf("RED: gitexec.%s = %q, want %q", name, got, value)
		}
	}
}

func TestC2293_002_NoProductionFileOutsideGitexecHoldsOriginMainAsAConstant(t *testing.T) {
	mod := moduleDir(t)
	for _, rel := range productionGoFiles(t, mod) {
		for _, name := range constantDeclsOf(parse(t, filepath.Join(mod, rel)), remoteMainValue) {
			t.Errorf("RED: %s declares %s = %q; the one home is gitexec.RemoteMain", rel, name, remoteMainValue)
		}
	}
}

func selectorsOfGitexec(f *ast.File) map[string]int {
	alias := ""
	for _, imp := range f.Imports {
		if p, _ := strconv.Unquote(imp.Path.Value); p == gitexecImport {
			alias = "gitexec"
			if imp.Name != nil {
				alias = imp.Name.Name
			}
		}
	}
	uses := map[string]int{}
	if alias == "" {
		return uses
	}
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == alias {
			uses[sel.Sel.Name]++
		}
		return true
	})
	return uses
}

func TestC2293_003_TheCoreAndInboxmoverCallersReadGitexecRemoteMain(t *testing.T) {
	mod := moduleDir(t)
	for _, rel := range []string{coreResumeBase, inboxmoverCurate} {
		f := parse(t, filepath.Join(mod, rel))
		if uses := selectorsOfGitexec(f)["RemoteMain"]; uses == 0 {
			t.Errorf("RED: %s never reads gitexec.RemoteMain", rel)
		}
		for _, lit := range stringLiterals(f) {
			if lit == remoteMainValue {
				t.Errorf("RED: %s still carries the literal %q", rel, remoteMainValue)
			}
		}
	}
}

func stringLiterals(f *ast.File) []string {
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if s, err := strconv.Unquote(lit.Value); err == nil {
				out = append(out, s)
			}
		}
		return true
	})
	return out
}

func TestC2293_004_TheTestsOfCoreAndInboxmoverAreUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	base, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", "origin/main")
	if err != nil || code != 0 {
		t.Skipf("no merge-base of HEAD with origin/main: code=%d err=%v", code, err)
	}
	out, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", strings.TrimSpace(base), "--", "go/internal/core", "go/internal/inboxmover")
	if err != nil || code != 0 {
		t.Fatalf("git diff exit=%d: %v %s", code, err, stderr)
	}
	for _, path := range strings.Fields(out) {
		if strings.HasSuffix(path, "_test.go") {
			t.Errorf("RED: %s changed; the criterion keeps these tests unchanged", path)
		}
	}
}

func TestC2293_005_TheCurateTestsPassOnTheNewConstant(t *testing.T) {
	mod := moduleDir(t)
	if got := gitexecConst(t, mod, "RemoteMain"); got != remoteMainValue {
		t.Fatalf("RED: gitexec.RemoteMain = %q, want %q", got, remoteMainValue)
	}
	out, _, code, err := acsassert.SubprocessOutput("go", "-C", mod, "test", "-count=1", "-run", "TestVerifyPremise_", "./internal/inboxmover")
	if err != nil || code != 0 {
		t.Errorf("RED: go test ./internal/inboxmover exit=%d: %v\n%s", code, err, acsassert.OutputTail(out, 15))
	}
}

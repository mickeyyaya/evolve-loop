//go:build acs

package cycle1864

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func parsePackage(t *testing.T, includeTests bool) (*token.FileSet, map[string]*ast.File) {
	t.Helper()
	dir := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "inboxmover")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		if !includeTests && strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files[name] = f
	}
	return fset, files
}

func funcDecls(files map[string]*ast.File) map[string][]*ast.FuncDecl {
	out := map[string][]*ast.FuncDecl{}
	for name, f := range files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				out[name] = append(out[name], fd)
			}
		}
	}
	return out
}

func TestC1864_001_RecordRootTaskFailureIsRetired(t *testing.T) {
	_, files := parsePackage(t, true)
	for name, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == "RecordRootTaskFailure" {
				t.Errorf("RED: %s still references RecordRootTaskFailure — the second quarantine writer is alive", name)
			}
			return true
		})
	}
}

func TestC1864_002_NoBumpLockOrVerifylockLeftInRootPackage(t *testing.T) {
	_, files := parsePackage(t, false)
	for name, f := range files {
		for _, imp := range f.Imports {
			if strings.HasSuffix(strings.Trim(imp.Path.Value, `"`), "/verifylock") {
				t.Errorf("RED: %s imports verifylock — only the dead quarantine writer used it", name)
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && strings.Contains(lit.Value, ".bump.lock") {
				t.Errorf("RED: %s still names the .bump.lock file of the dead writer", name)
			}
			return true
		})
	}
}

func TestC1864_003_ClaimLaneScopeReturnsOnlyTheClaimedIds(t *testing.T) {
	fn := reflect.ValueOf(inboxmover.ClaimLaneScope)
	typ := fn.Type()
	if typ.NumOut() != 1 {
		t.Fatalf("RED: ClaimLaneScope returns %d values, want 1 ([]string)", typ.NumOut())
	}
	if typ.Out(0) != reflect.TypeOf([]string(nil)) {
		t.Fatalf("RED: ClaimLaneScope result type = %v, want []string", typ.Out(0))
	}
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"present-a", "present-b"} {
		if err := os.WriteFile(filepath.Join(inbox, id+".json"), []byte(`{"id":"`+id+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	opts := inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}
	out := fn.Call([]reflect.Value{reflect.ValueOf(opts), reflect.ValueOf(5), reflect.ValueOf([]string{"present-a", "missing", "present-b", "present-a"})})
	claimed := out[0].Interface().([]string)
	sort.Strings(claimed)
	if strings.Join(claimed, ",") != "present-a,present-b" {
		t.Errorf("claimed = %v, want [present-a present-b] (missing skipped, duplicate collapsed)", claimed)
	}
	for _, id := range []string{"present-a", "present-b"} {
		if _, err := os.Stat(filepath.Join(inbox, "processing", "cycle-5", id+".json")); err != nil {
			t.Errorf("%s not moved into processing/cycle-5: %v", id, err)
		}
	}
	if _, err := os.Stat(filepath.Join(inbox, "missing.json")); err == nil {
		t.Errorf("missing id appeared in the inbox")
	}
}

func alwaysNilErrorFuncs(files map[string]*ast.File) []string {
	var flagged []string
	for name, decls := range funcDecls(files) {
		for _, fd := range decls {
			res := fd.Type.Results
			if fd.Body == nil || res == nil || len(res.List) == 0 {
				continue
			}
			last := res.List[len(res.List)-1]
			if id, ok := last.Type.(*ast.Ident); !ok || id.Name != "error" || len(last.Names) > 0 {
				continue
			}
			returns, allNil := 0, true
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if _, lit := n.(*ast.FuncLit); lit {
					return false
				}
				rs, ok := n.(*ast.ReturnStmt)
				if !ok {
					return true
				}
				returns++
				if len(rs.Results) == 0 {
					allNil = false
					return true
				}
				if id, ok := rs.Results[len(rs.Results)-1].(*ast.Ident); !ok || id.Name != "nil" {
					allNil = false
				}
				return true
			})
			if returns > 0 && allNil {
				flagged = append(flagged, name+":"+fd.Name.Name)
			}
		}
	}
	sort.Strings(flagged)
	return flagged
}

func TestC1864_004_NoFunctionReturnsAnAlwaysNilError(t *testing.T) {
	_, files := parsePackage(t, false)
	if flagged := alwaysNilErrorFuncs(files); len(flagged) > 0 {
		t.Errorf("RED: functions whose every return yields a nil error: %v", flagged)
	}
}

func TestC1864_005_AlwaysNilDetectorFlagsAnInvariantNilError(t *testing.T) {
	src := "package p\nfunc bad() (int, error) { return 1, nil }\nfunc good() (int, error) { if true { return 0, nil }; return 0, io.EOF }\nfunc noerr() int { return 1 }\n"
	f, err := parser.ParseFile(token.NewFileSet(), "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := alwaysNilErrorFuncs(map[string]*ast.File{"p.go": f})
	if len(got) != 1 || got[0] != "p.go:bad" {
		t.Errorf("detector flagged %v, want exactly [p.go:bad]", got)
	}
}

func skippingTests(files map[string]*ast.File) map[string]string {
	out := map[string]string{}
	for name, decls := range funcDecls(files) {
		if !strings.HasSuffix(name, "_test.go") {
			continue
		}
		for _, fd := range decls {
			if fd.Body == nil {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch sel.Sel.Name {
				case "Skip", "Skipf", "SkipNow":
					if recv, ok := sel.X.(*ast.Ident); ok && recv.Name != "filepath" {
						out[fd.Name.Name] = name
					}
				}
				return true
			})
		}
	}
	return out
}

func TestC1864_006_MissingProcessingDirTestNoLongerSkips(t *testing.T) {
	_, files := parsePackage(t, true)
	skips := skippingTests(files)
	if file, ok := skips["TestReleaseCycleProcessing_EntirelyMissingProcessingDir"]; ok {
		t.Errorf("RED: TestReleaseCycleProcessing_EntirelyMissingProcessingDir still skips (%s)", file)
	}
	var found bool
	for _, decls := range funcDecls(files) {
		for _, fd := range decls {
			found = found || fd.Name.Name == "TestReleaseCycleProcessing_EntirelyMissingProcessingDir"
		}
	}
	if !found {
		t.Errorf("the test must be kept, not deleted")
	}
}

func TestC1864_007_EveryRemainingSkipIsNamedByTheDecompositionDoc(t *testing.T) {
	_, files := parsePackage(t, true)
	doc, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "decomposition", "06-inboxmover.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(doc)
	for fn := range skippingTests(files) {
		if !strings.Contains(text, fn) {
			t.Errorf("RED: %s skips but 06-inboxmover.md does not name it", fn)
		}
	}
	if strings.Contains(text, "RecordRootTaskFailure") && !strings.Contains(text, "06-F2") {
		t.Errorf("doc mentions RecordRootTaskFailure without recording the 06-F2 resolution")
	}
	if strings.Contains(text, "STAYS") && strings.Contains(text, "root_failure.go (`RecordRootTaskFailure` :39-72") {
		t.Errorf("RED: doc still lists root_failure.go as a STAYS row")
	}
}

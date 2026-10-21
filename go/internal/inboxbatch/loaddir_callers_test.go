package inboxbatch

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoProductionCodeCallsLoadDir(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	skip := map[string]bool{filepath.Join(root, "acs"): true, filepath.Join(root, "vendor"): true}
	var callers []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skip[path] || d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, perr := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if perr != nil {
			return perr
		}
		if callsLoadDirAnywhere(file) {
			callers = append(callers, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(callers) != 0 {
		t.Fatalf("these files call LoadDir, whose text projection loses the unreadable flag; read ScanDir and name each DirScan.Warnings entry: %v", callers)
	}
}

func callsLoadDirAnywhere(file *ast.File) bool {
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && callsLoadDir(call) {
			found = true
		}
		return !found
	})
	return found
}

func callsLoadDir(call *ast.CallExpr) bool {
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		return fn.Sel.Name == "LoadDir"
	case *ast.Ident:
		return fn.Name == "LoadDir"
	}
	return false
}

func TestCallsLoadDirAnywhere_SeesEveryFormOfTheCall(t *testing.T) {
	t.Parallel()
	for src, want := range map[string]bool{
		"package p\nfunc f() { items, _, err := inboxbatch.LoadDir(d); _, _ = items, err }":        true,
		"package p\nfunc f() { items, w, err := inboxbatch.LoadDir(d); _ = w; _, _ = items, err }": true,
		"package p\nvar a, _, b = LoadDir(d)":                                                      true,
		"package p\nfunc f() { LoadDir(d) }":                                                       true,
		"package p\nfunc f() []Item { items, _, _ := inboxbatch.LoadDir(d); return items }":        true,
		"package p\nfunc f() { scan, err := inboxbatch.ScanDir(d); _, _ = scan, err }":             false,
		"package p\nfunc f() { a, _ := x.y.LoadDirs(d); _ = a }":                                   false,
		"package p\nfunc f() { a, _ := fns[0](d); _ = a }":                                         false,
	} {
		file, err := parser.ParseFile(token.NewFileSet(), "x.go", src, 0)
		if err != nil {
			t.Fatal(err)
		}
		if got := callsLoadDirAnywhere(file); got != want {
			t.Errorf("callsLoadDirAnywhere(%q) = %t, want %t", src, got, want)
		}
	}
}

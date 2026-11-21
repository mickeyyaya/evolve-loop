package journeys

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	ErrNotFound = errors.New("no test of that name is declared")
	ErrTagGated = errors.New("the test sits in a file that the default build skips")
)

var ciBuild = build.Context{GOOS: "linux", GOARCH: "amd64", Compiler: "gc", CgoEnabled: true, ReleaseTags: build.Default.ReleaseTags}

func Resolve(root, test string) (string, error) {
	dot := strings.LastIndex(test, ".")
	if dot < 0 || !testRefRE.MatchString(test) {
		return "", fmt.Errorf("%w: %s", ErrNotFound, test)
	}
	pkg, name := test[:dot], test[dot+1:]
	dir := filepath.Join(root, "go", filepath.FromSlash(pkg))
	entries, _ := os.ReadDir(dir)
	gated := false
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), "_test.go") || !declares(filepath.Join(dir, e.Name()), name) {
			continue
		}
		if ok, _ := ciBuild.MatchFile(dir, e.Name()); ok {
			return path.Join("go", pkg, e.Name()), nil
		}
		gated = true
	}
	if gated {
		return "", fmt.Errorf("%w: %s", ErrTagGated, test)
	}
	return "", fmt.Errorf("%w: %s", ErrNotFound, test)
}

func declares(file, name string) bool {
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
	if err != nil {
		return false
	}
	for _, d := range parsed.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == name {
			return true
		}
	}
	return false
}

func Unresolved(root string, rows []Row) []string {
	var out []string
	for _, r := range rows {
		if r.Retired != "" {
			continue
		}
		if _, err := Resolve(root, r.Test); err != nil {
			out = append(out, r.ID+": "+err.Error())
		}
	}
	return out
}

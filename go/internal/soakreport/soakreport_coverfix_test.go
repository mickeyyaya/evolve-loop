package soakreport

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var filesystemMutators = map[string]bool{
	"WriteFile": true, "Create": true, "CreateTemp": true, "Remove": true, "RemoveAll": true,
	"Rename": true, "Mkdir": true, "MkdirAll": true, "MkdirTemp": true, "OpenFile": true,
	"Truncate": true, "Chmod": true, "Chown": true, "Chtimes": true, "Symlink": true, "Link": true,
}

func TestSoakReport_NeverMutatesTheFilesystem(t *testing.T) {
	t.Parallel()
	sources, err := filepath.Glob("*.go")
	if err != nil || len(sources) == 0 {
		t.Fatalf("no package sources found: %v", err)
	}
	fset := token.NewFileSet()
	for _, src := range sources {
		if strings.HasSuffix(src, "_test.go") {
			continue
		}
		file, perr := parser.ParseFile(fset, src, nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", src, perr)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if ok && pkg.Name == "os" && filesystemMutators[sel.Sel.Name] {
				t.Errorf("%s: os.%s mutates the filesystem, but the soak reporter only accounts", fset.Position(sel.Pos()), sel.Sel.Name)
			}
			return true
		})
	}
}

func TestCollect_LeavesTheWorkspaceUntouched(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	plantWS(t, root, 5, map[string]string{
		"phase-timing.json":         `[{"phase":"ship","verdict":"PASS"}]`,
		"build-interactions.ndjson": `{"kind":"salvage","result":"would_act"}` + "\n",
	})
	before := snapshotTree(t, root)
	Collect(root, []int{5}).Render()
	after := snapshotTree(t, root)
	if before != after {
		t.Errorf("Collect and Render changed the project tree:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func snapshotTree(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		b.WriteString(p + " " + info.ModTime().String() + " " + info.Mode().String() + "\n")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}

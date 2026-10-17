package structure

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func testFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "testdata" || d.Name() == "vendor") {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(path, "_test.go") {
			rel, err := filepath.Rel(root, path)
			files = append(files, rel)
			return err
		}
		return nil
	})
	return files, err
}

func rawBareClones(root, rel string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(root, rel), nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var sites []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		var lits []string
		for _, arg := range call.Args {
			if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if s, err := strconv.Unquote(lit.Value); err == nil {
					lits = append(lits, s)
				}
			}
		}
		if slices.Contains(lits, "clone") && slices.Contains(lits, "--bare") {
			sites = append(sites, rel+":"+strconv.Itoa(fset.Position(call.Pos()).Line))
		}
		return true
	})
	return sites, nil
}

func rawBareCloneSites(root string) ([]string, error) {
	files, err := testFiles(root)
	if err != nil {
		return nil, err
	}
	var sites []string
	for _, rel := range files {
		found, err := rawBareClones(root, rel)
		if err != nil {
			return nil, err
		}
		sites = append(sites, found...)
	}
	return sites, nil
}

func TestNoTestRunsARawBareClone(t *testing.T) {
	t.Parallel()
	sites, err := rawBareCloneSites(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("rawBareCloneSites = %v", err)
	}
	if len(sites) > 0 {
		t.Errorf("%d raw bare clone(s); a bare clone carries no test identity, so use gittest Repo.CloneBare:\n%s", len(sites), strings.Join(sites, "\n"))
	}
}

func TestRawBareCloneSites_FindsACloneWhateverTheCallee(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"a/a_test.go":          "package a\nfunc f() {\n\tr.Git(\"clone\", \"-q\", \"--bare\", \".\", d)\n\tgitIn(t, d, \"clone\", \"-q\", \"--bare\", o, s)\n}\n",
		"a/ok_test.go":         "package a\nfunc f() {\n\tr.Git(\"clone\", \"-q\", o, d)\n\tr.Git(\"init\", \"--bare\")\n\tr.CloneBare(o, d)\n}\n",
		"a/plain.go":           "package a\nfunc f() {\n\tr.Git(\"clone\", \"--bare\")\n}\n",
		"a/testdata/x_test.go": "package a\nfunc f() {\n\tr.Git(\"clone\", \"--bare\")\n}\n",
	})

	sites, err := rawBareCloneSites(root)

	want := []string{filepath.Join("a", "a_test.go") + ":3", filepath.Join("a", "a_test.go") + ":4"}
	slices.Sort(sites)
	if err != nil || !slices.Equal(sites, want) {
		t.Errorf("rawBareCloneSites = %q, %v, want %q", sites, err, want)
	}
}

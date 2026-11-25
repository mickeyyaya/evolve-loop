package policy_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const diskFloorResolverDir = "internal/diskfloor"

var diskFloorFieldNames = []string{"DiskFloorGB", "DiskWarnGB", "MinFreeGiB", "FloorGB", "WarnGB"}

var diskFloorKeyLiterals = []string{"disk_floor_gb", "disk_warn_gb", "min_free_gib"}

func diskFloorReaders(moduleRoot string) ([]string, error) {
	var found []string
	err := filepath.WalkDir(moduleRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == diskFloorResolverDir || rel == "acs" || d.Name() == "testdata" || d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		hits, err := diskFloorReadsInFile(path)
		if err != nil {
			return err
		}
		for _, h := range hits {
			found = append(found, rel+": "+h)
		}
		return nil
	})
	return found, err
}

func diskFloorReadsInFile(path string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	tags := map[*ast.BasicLit]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		if f, ok := n.(*ast.Field); ok && f.Tag != nil {
			tags[f.Tag] = true
		}
		return true
	})
	var hits []string
	ast.Inspect(file, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.SelectorExpr:
			if slices.Contains(diskFloorFieldNames, v.Sel.Name) {
				hits = append(hits, fset.Position(v.Pos()).String()+" reads ."+v.Sel.Name)
			}
		case *ast.KeyValueExpr:
			if id, ok := v.Key.(*ast.Ident); ok && slices.Contains(diskFloorFieldNames, id.Name) {
				hits = append(hits, fset.Position(v.Pos()).String()+" sets "+id.Name)
			}
		case *ast.BasicLit:
			if v.Kind != token.STRING || tags[v] {
				return true
			}
			text, uerr := strconv.Unquote(v.Value)
			if uerr != nil {
				return true
			}
			for _, key := range diskFloorKeyLiterals {
				if strings.Contains(text, key) {
					hits = append(hits, fset.Position(v.Pos()).String()+" names the key "+key)
				}
			}
		}
		return true
	})
	return hits, nil
}

func diskFloorWriteTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSingleReader_NoDiskFloorReaderLivesOutsideTheResolver(t *testing.T) {
	t.Parallel()
	readers, err := diskFloorReaders(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if len(readers) > 0 {
		t.Errorf("%d reader(s) of a disk floor outside %s; route each through the resolver:\n%s", len(readers), diskFloorResolverDir, strings.Join(readers, "\n"))
	}
}

func TestSingleReader_AGuardFindsASecondReaderOutsideTheResolver(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		files   map[string]string
		wantIn  []string
		wantOut []string
	}{
		{
			name: "a selector read of an old floor field",
			files: map[string]string{
				"internal/diskfloor/resolver.go": "package diskfloor\n\nfunc F(p struct{ DiskFloorGB int }) int { return p.DiskFloorGB }\n",
				"internal/other/other.go":        "package other\n\nfunc F(p struct{ DiskFloorGB int }) int { return p.DiskFloorGB }\n",
				"internal/clean/clean.go":        "package clean\n\nfunc F() int { return 1 }\n",
			},
			wantIn:  []string{"internal/other/other.go"},
			wantOut: []string{"internal/diskfloor/resolver.go", "internal/clean/clean.go"},
		},
		{
			name: "a second reader of the new floor field",
			files: map[string]string{
				"internal/second/second.go": "package second\n\nfunc F(b struct{ FloorGB float64 }) float64 { return b.FloorGB }\n",
			},
			wantIn: []string{"internal/second/second.go"},
		},
		{
			name: "a policy key literal outside a struct tag",
			files: map[string]string{
				"internal/lit/lit.go":   "package lit\n\nconst key = \"preflight.min_free_gib\"\n",
				"internal/tagged/t.go":  "package tagged\n\ntype P struct {\n\tX float64 `json:\"min_free_gib\"`\n}\n",
				"internal/ctor/ctor.go": "package ctor\n\ntype P struct{ MinFreeGiB float64 }\n\nvar v = P{MinFreeGiB: 2}\n",
			},
			wantIn:  []string{"internal/lit/lit.go", "internal/ctor/ctor.go"},
			wantOut: []string{"internal/tagged/t.go"},
		},
		{
			name: "a test file and a testdata tree are not readers",
			files: map[string]string{
				"internal/x/x_test.go":     "package x\n\nfunc f(p struct{ DiskFloorGB int }) int { return p.DiskFloorGB }\n",
				"internal/x/testdata/d.go": "package d\n\nfunc f(p struct{ DiskFloorGB int }) int { return p.DiskFloorGB }\n",
				"acs/cycle1/predicates.go": "package c\n\nfunc f(p struct{ DiskFloorGB int }) int { return p.DiskFloorGB }\n",
			},
			wantOut: []string{"x_test.go", "testdata", "acs/"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			diskFloorWriteTree(t, root, c.files)
			readers, err := diskFloorReaders(root)
			if err != nil {
				t.Fatal(err)
			}
			joined := strings.Join(readers, "\n")
			for _, want := range c.wantIn {
				if !strings.Contains(joined, want) {
					t.Errorf("guard missed the reader in %s; found:\n%s", want, joined)
				}
			}
			for _, not := range c.wantOut {
				if strings.Contains(joined, not) {
					t.Errorf("guard flagged %s, which is no reader; found:\n%s", not, joined)
				}
			}
		})
	}
}

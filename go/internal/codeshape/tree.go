package codeshape

import (
	"go/ast"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const windowSize = 5

type Package struct {
	Dir             string
	Lines, Exported int
}

type Tree struct {
	Funcs    []Func
	Packages map[string]*Package
	Windows  map[string]int
	Lines    int
}

func Scan(goRoot string) (Tree, error) {
	t := Tree{Packages: map[string]*Package{}, Windows: map[string]int{}}
	err := filepath.WalkDir(goRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(goRoot, p)
		if d.IsDir() && rel != "." && skippedDir(d.Name()) {
			return fs.SkipDir
		}
		if d.IsDir() || !Production(filepath.ToSlash(rel)) {
			return nil
		}
		return t.add(p, filepath.ToSlash(rel))
	})
	return t, err
}

func (t *Tree) add(path, rel string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	f, fset, err := parseFile(rel, src)
	if err != nil {
		return err
	}
	t.Funcs = append(t.Funcs, funcsOf(rel, f, fset)...)
	lines := LogicalLines(string(src))
	for _, w := range Windows(lines) {
		t.Windows[w]++
	}
	t.Lines += len(lines)
	dir := filepath.ToSlash(filepath.Dir(rel))
	if t.Packages[dir] == nil {
		t.Packages[dir] = &Package{Dir: dir}
	}
	t.Packages[dir].Lines += len(lines)
	t.Packages[dir].Exported += exportedCount(f)
	return nil
}

func LogicalLines(src string) []string {
	var out []string
	for _, l := range strings.Split(src, "\n") {
		s := strings.TrimSpace(l)
		if s == "" || s == "}" || s == ")" || s == "{" || strings.HasPrefix(s, "import") || strings.HasPrefix(s, "package ") {
			continue
		}
		out = append(out, s)
	}
	return out
}

func Windows(lines []string) []string {
	var out []string
	for i := 0; i+windowSize <= len(lines); i++ {
		out = append(out, strings.Join(lines[i:i+windowSize], "\n"))
	}
	return out
}

func exportedCount(f *ast.File) int {
	n := 0
	for _, d := range f.Decls {
		switch x := d.(type) {
		case *ast.FuncDecl:
			if x.Name.IsExported() && (x.Recv == nil || ast.IsExported(typeName(x.Recv.List[0].Type))) {
				n++
			}
		case *ast.GenDecl:
			n += exportedSpecs(x.Specs)
		}
	}
	return n
}

func exportedSpecs(specs []ast.Spec) int {
	n := 0
	for _, s := range specs {
		switch y := s.(type) {
		case *ast.TypeSpec:
			if y.Name.IsExported() {
				n++
			}
		case *ast.ValueSpec:
			for _, name := range y.Names {
				if name.IsExported() {
					n++
				}
			}
		}
	}
	return n
}

func (t Tree) DuplicateWindows() int {
	n := 0
	for _, count := range t.Windows {
		if count >= 2 {
			n += count
		}
	}
	return n
}

func (t Tree) DiffDuplicates(added [][]string) (dup, lines int) {
	for _, hunk := range added {
		logical := LogicalLines(strings.Join(hunk, "\n"))
		lines += len(logical)
		for _, w := range Windows(logical) {
			if t.Windows[w] >= 2 {
				dup++
			}
		}
	}
	return dup, lines
}

func (t Tree) Depths() []float64 {
	var out []float64
	for dir, p := range t.Packages {
		if strings.HasPrefix(dir, "internal/") && p.Exported > 0 {
			out = append(out, float64(p.Lines)/float64(p.Exported))
		}
	}
	return out
}

func skippedDir(name string) bool {
	return name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".")
}

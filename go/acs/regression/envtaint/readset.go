//go:build acs

package envtaint

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ipcAllowedMarker is the in-source annotation that designates a split-const or
// constant EVOLVE_ key as an IPC-protocol value (writer-injected, parent->child),
// NOT an operator dial. A key whose declaration carries this marker is excluded
// from the read-set everywhere it is used — by value — because an IPC key is IPC
// at every read site. The marker lives in source files that are PROTECTED
// surfaces (ADR-0064 Pillar 1), so an autonomous cycle cannot add one to dodge.
//
// We match the stable token only, so both spellings in the tree are honored:
// "SSOT IPC-protocol-allowed" and "SSOT §IPC-protocol-allowed".
const ipcAllowedMarker = "IPC-protocol-allowed"

var flagNameRE = regexp.MustCompile(`^EVOLVE(_[A-Z0-9]+)+$`)

func EvolveConstKeys(src string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "src.go", src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	keys := map[string]bool{}
	collectEvolveConstKeys(fset, file.Name.Name, []*ast.File{file}, keys)
	return sortedKeys(keys), nil
}

func collectEvolveConstKeys(fset *token.FileSet, pkgName string, files []*ast.File, keys map[string]bool) {
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	conf := &types.Config{Importer: stubImporter{}, Error: func(error) {}}
	_, _ = conf.Check(pkgName, fset, files, info)

	markerCovered := map[int]bool{}
	for _, f := range files {
		for line := range markerCoveredLines(fset, f) {
			markerCovered[line] = true
		}
	}

	all := map[string]bool{}
	excluded := map[string]bool{}

	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			gd, ok := n.(*ast.GenDecl)
			if !ok {
				return true
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || !(commentHasMarker(gd.Doc) || commentHasMarker(vs.Doc) || commentHasMarker(vs.Comment)) {
					continue
				}
				for _, val := range vs.Values {
					if v, ok := foldedFlagName(info, val); ok {
						excluded[v] = true
					}
				}
			}
			return true
		})
	}

	for expr, tv := range info.Types {
		if tv.Value == nil || tv.Value.Kind() != constant.String {
			continue
		}
		v := constant.StringVal(tv.Value)
		if !flagNameRE.MatchString(v) {
			continue
		}
		all[v] = true
		if markerCovered[fset.Position(expr.Pos()).Line] {
			excluded[v] = true
		}
	}
	for v := range all {
		if !excluded[v] {
			keys[v] = true
		}
	}
}

func commentHasMarker(cg *ast.CommentGroup) bool {
	return cg != nil && strings.Contains(cg.Text(), ipcAllowedMarker)
}

func foldedFlagName(info *types.Info, e ast.Expr) (string, bool) {
	tv, ok := info.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	s := constant.StringVal(tv.Value)
	if !flagNameRE.MatchString(s) {
		return "", false
	}
	return s, true
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

var readSetSkipDirs = map[string]bool{
	"vendor": true, "testdata": true, ".git": true, "node_modules": true,
	".evolve": true, "ipcenv": true, "acs": true,
}

const registryTableSuffix = "internal/flagregistry/registry_table.go"

func ReadSet(goRoot string) (keys []string, skipped []string, err error) {
	set := map[string]bool{}
	skipped, err = forEachProductionPackage(goRoot, func(fset *token.FileSet, pkgName string, files []*ast.File) {
		collectEvolveConstKeys(fset, pkgName, files, set)
	})
	if err != nil {
		return nil, skipped, err
	}
	return sortedKeys(set), skipped, nil
}

func forEachProductionPackage(goRoot string, fn func(fset *token.FileSet, pkgName string, files []*ast.File)) (skipped []string, err error) {
	fset := token.NewFileSet()
	byDir := map[string][]*ast.File{}
	var dirOrder []string

	walkErr := filepath.Walk(goRoot, func(path string, fi os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if fi.IsDir() {
			if readSetSkipDirs[fi.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if strings.HasSuffix(filepath.ToSlash(path), registryTableSuffix) {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			skipped = append(skipped, path)
			return nil
		}
		dir := filepath.Dir(path)
		if _, seen := byDir[dir]; !seen {
			dirOrder = append(dirOrder, dir)
		}
		byDir[dir] = append(byDir[dir], f)
		return nil
	})
	if walkErr != nil {
		return skipped, fmt.Errorf("walk %s: %w", goRoot, walkErr)
	}

	for _, dir := range dirOrder {
		files := byDir[dir]
		fn(fset, files[0].Name.Name, files)
	}
	return skipped, nil
}

func markerCoveredLines(fset *token.FileSet, file *ast.File) map[int]bool {
	covered := map[int]bool{}
	for _, cg := range file.Comments {
		if !strings.Contains(cg.Text(), ipcAllowedMarker) {
			continue
		}
		start := fset.Position(cg.Pos()).Line
		end := fset.Position(cg.End()).Line
		for l := start; l <= end+1; l++ {
			covered[l] = true
		}
	}
	return covered
}

type stubImporter struct{}

func (stubImporter) Import(path string) (*types.Package, error) {
	name := path
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		name = path[i+1:]
	}
	return types.NewPackage(path, name), nil
}

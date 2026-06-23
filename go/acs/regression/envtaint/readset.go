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

// flagNameRE matches a complete, well-formed flag name (anchored, so it never
// matches a concat operand like "EVOLVE_" or a dynamic prefix like
// "EVOLVE_PHASE_" that ends in '_'). Mirrors the flagreaders guard's literal
// match, but applied to the type-checker's FOLDED constant value.
var flagNameRE = regexp.MustCompile(`^EVOLVE(_[A-Z0-9]+)+$`)

// EvolveConstKeys returns the sorted, de-duplicated set of EVOLVE_* operator-dial
// keys that the source reads as compile-time constants — the read-set R for one
// file. It folds split-consts (so the cycle-20 dodge "EVOLVE_"+"X" is visible),
// excludes keys whose declaration carries the IPC-allowed marker, and excludes
// dynamic (non-constant) keys.
//
// Type-checking is best-effort: imports are stubbed and errors are swallowed, so
// a whole-repo walk can fold every file without resolving the build graph (an
// unresolvable "os" does not abort the constant fold of an argument).
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

// collectEvolveConstKeys best-effort type-checks a package's files together (so
// same-package cross-file constant references resolve) and adds every
// operator-dial EVOLVE_ key it folds into keys. Marker-covered and dynamic keys
// are excluded.
func collectEvolveConstKeys(fset *token.FileSet, pkgName string, files []*ast.File, keys map[string]bool) {
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	conf := &types.Config{Importer: stubImporter{}, Error: func(error) {}}
	// Best-effort: ignore the aggregate error; info.Types is still populated
	// with every constant we can fold.
	_, _ = conf.Check(pkgName, fset, files, info)

	markerCovered := map[int]bool{}
	for _, f := range files {
		for line := range markerCoveredLines(fset, f) {
			markerCovered[line] = true
		}
	}

	all := map[string]bool{}
	excluded := map[string]bool{}

	// (a) Declaration-associated marker: a const/var spec whose doc or trailing
	// comment (or its GenDecl's doc) carries the IPC-allowed marker excludes
	// every EVOLVE_ value it defines, BY VALUE — so the key is excluded at every
	// read site, however far a use is from the marked declaration. This is the
	// robust path (an AST doc-comment block of any height is associated with its
	// spec), where raw line-proximity is not.
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

	// (b) One pass over every folded EVOLVE_ flag value; an inline/trailing
	// marker (e.g. above a subprocess-env slice element, not a declaration) is
	// caught by line proximity and also excludes by value.
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

// commentHasMarker reports whether a comment group carries the IPC-allowed marker.
func commentHasMarker(cg *ast.CommentGroup) bool {
	return cg != nil && strings.Contains(cg.Text(), ipcAllowedMarker)
}

// foldedFlagName returns e's folded value when it is a well-formed EVOLVE_ flag
// name constant.
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

// readSetSkipDirs mirrors the flagreaders guard's non-production exclusions so
// the read-set scans the same surface the orphan gate does. ipcenv is the IPC
// protocol SSOT (its literals ARE the protocol, not operator dials); acs is the
// gate's own tree.
var readSetSkipDirs = map[string]bool{
	"vendor": true, "testdata": true, ".git": true, "node_modules": true,
	".evolve": true, "ipcenv": true, "acs": true,
}

// registryTableSuffix is the repo-relative tail of the flag catalog file, which
// the read-set excludes (it is the SSOT being checked, not a reader).
const registryTableSuffix = "internal/flagregistry/registry_table.go"

// ReadSet walks production Go under goRoot (non-test, outside the skip dirs) and
// returns the sorted union of operator-dial EVOLVE_ keys read across it. Files
// are grouped by directory and type-checked per package so same-package
// cross-file constants resolve. Unparseable files are skipped (the compiler
// catches syntax errors elsewhere); the returned skipped list keeps that loud.
func ReadSet(goRoot string) (keys []string, skipped []string, err error) {
	fset := token.NewFileSet()
	set := map[string]bool{}
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
		// The registry data file is the flag CATALOG (its {Name: "EVOLVE_..."}
		// literals are every flag name by design), not a reader — scanning it
		// would re-assert the registry against itself, exactly as flagreaders
		// excludes control-flags.md.
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
		return nil, skipped, fmt.Errorf("walk %s: %w", goRoot, walkErr)
	}

	for _, dir := range dirOrder {
		files := byDir[dir]
		collectEvolveConstKeys(fset, files[0].Name.Name, files, set)
	}
	return sortedKeys(set), skipped, nil
}

// markerCoveredLines returns the lines a marker comment annotates: every line of
// a marker-bearing comment group PLUS the line immediately after it (the
// annotated statement or declaration). An *ast.CommentGroup is already a
// contiguous block, so this handles a trailing marker, a single line above, and
// an N-line comment block above uniformly — without a fixed proximity window.
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

// stubImporter resolves every import path to an empty package, so best-effort
// type-checking proceeds (and folds constants) without the build graph. Selector
// uses against a stub (e.g. os.Getenv) error out, but those errors are swallowed
// and never block the argument fold.
type stubImporter struct{}

func (stubImporter) Import(path string) (*types.Package, error) {
	name := path
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		name = path[i+1:]
	}
	return types.NewPackage(path, name), nil
}

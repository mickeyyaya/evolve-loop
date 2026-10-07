package repocontract

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet"
)

func TestPackages_HoldEveryTestThatReadsTheWholeTree(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	files, _, err := rawgitratchet.BoundTestFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	reading := treeReadingTests(t, root, files)
	readers := slices.Sorted(maps.Keys(reading))
	for _, want := range []string{"internal/repocontract", "internal/sizeratchet", "internal/policy", "internal/acssuite", "internal/guards"} {
		if !slices.Contains(readers, want) {
			t.Fatalf("the detector must find %s; it found %v", want, readers)
		}
	}
	for _, problem := range packProblems(reading, Packages(), TreeReadingTests()) {
		t.Error(problem)
	}
}

func packProblems(reading map[string][]string, pack []string, selections []TestSelection) []string {
	inPack := map[string]bool{}
	for _, pattern := range pack {
		inPack[strings.TrimSuffix(strings.TrimPrefix(pattern, "./"), "/...")] = true
	}
	selected := map[string][]string{}
	for _, selection := range selections {
		dir := strings.TrimPrefix(selection.Package, "./")
		selected[dir] = append(selected[dir], selection.Tests...)
	}
	var problems []string
	for _, dir := range slices.Sorted(maps.Keys(reading)) {
		if inPack[dir] {
			continue
		}
		if len(reading[dir]) == 0 {
			problems = append(problems, fmt.Sprintf("%s reads the whole tree outside any test the detector can name; put it in Packages()", dir))
		}
		for _, test := range reading[dir] {
			if !slices.Contains(selected[dir], test) {
				problems = append(problems, fmt.Sprintf("%s.%s reads the whole tree, so a lane that touches neither its package nor an importer can still break it; add it to TreeReadingTests() or put the package in Packages()", dir, test))
			}
		}
	}
	for _, dir := range slices.Sorted(maps.Keys(selected)) {
		for _, test := range selected[dir] {
			if inPack[dir] || !slices.Contains(reading[dir], test) {
				problems = append(problems, fmt.Sprintf("%s.%s is selected by name but its package is in the pack or it no longer reads the tree; remove it from TreeReadingTests()", dir, test))
			}
		}
	}
	return problems
}

func treeReadingTests(t *testing.T, root string, files []string) map[string][]string {
	t.Helper()
	byDir := map[string][]*ast.File{}
	for _, rel := range files {
		if strings.HasPrefix(rel, "acs/") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, filepath.FromSlash(rel)), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		byDir[path.Dir(rel)] = append(byDir[path.Dir(rel)], file)
	}
	reading := map[string][]string{}
	for dir, parsed := range byDir {
		if tests, reads := readingTestsOf(parsed, dir); reads {
			reading[dir] = tests
		}
	}
	return reading
}

type declUnit struct {
	names []string
	test  bool
	reads bool
	refs  map[string]bool
}

func readingTestsOf(files []*ast.File, dir string) (tests []string, reads bool) {
	units := declUnits(files, dir)
	reading := map[string]bool{}
	for _, unit := range units {
		if unit.reads {
			reads = true
			markAll(reading, unit.names)
		}
	}
	for grew := true; grew; {
		grew = false
		for _, unit := range units {
			if !allIn(reading, unit.names) && anyIn(unit.refs, reading) {
				markAll(reading, unit.names)
				grew = true
			}
		}
	}
	for _, unit := range units {
		if unit.test && reading[unit.names[0]] && !slices.Contains(tests, unit.names[0]) {
			tests = append(tests, unit.names[0])
		}
	}
	slices.Sort(tests)
	return tests, reads
}

func declUnits(files []*ast.File, dir string) []declUnit {
	var units []declUnit
	for _, file := range files {
		for _, decl := range file.Decls {
			for _, node := range declNodes(decl) {
				units = append(units, declUnit{
					names: declaredNames(node),
					test:  isGoTest(node),
					reads: climbsOutOfItsPackage(node, dir) || walksUpToGoMod(node),
					refs:  identifiersIn(node),
				})
			}
		}
	}
	return units
}

func declNodes(decl ast.Decl) []ast.Node {
	gen, ok := decl.(*ast.GenDecl)
	if !ok {
		return []ast.Node{decl}
	}
	var nodes []ast.Node
	for _, spec := range gen.Specs {
		if _, isImport := spec.(*ast.ImportSpec); !isImport {
			nodes = append(nodes, spec)
		}
	}
	return nodes
}

func declaredNames(node ast.Node) []string {
	var idents []*ast.Ident
	switch decl := node.(type) {
	case *ast.FuncDecl:
		idents = []*ast.Ident{decl.Name}
	case *ast.ValueSpec:
		idents = decl.Names
	case *ast.TypeSpec:
		idents = []*ast.Ident{decl.Name}
	}
	var names []string
	for _, id := range idents {
		if id.Name != "_" {
			names = append(names, id.Name)
		}
	}
	return names
}

func isGoTest(node ast.Node) bool {
	fn, ok := node.(*ast.FuncDecl)
	if !ok || fn.Recv != nil || fn.Name.Name == "TestMain" || fn.Type.Params.NumFields() != 1 {
		return false
	}
	rest, ok := strings.CutPrefix(fn.Name.Name, "Test")
	return ok && (rest == "" || !unicode.IsLower(rune(rest[0])))
}

func identifiersIn(node ast.Node) map[string]bool {
	refs := map[string]bool{}
	ast.Inspect(node, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			refs[id.Name] = true
		}
		return true
	})
	return refs
}

func markAll(set map[string]bool, names []string) {
	for _, name := range names {
		set[name] = true
	}
}

func allIn(set map[string]bool, names []string) bool {
	return !slices.ContainsFunc(names, func(name string) bool { return !set[name] })
}

func anyIn(refs, set map[string]bool) bool {
	for name := range refs {
		if set[name] {
			return true
		}
	}
	return false
}

func climbsOutOfItsPackage(node ast.Node, dir string) bool {
	depth := depthBelowTheModuleRoot(dir)
	return inspectFound(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return false
		}
		steps, rest := climb(call)
		landing := depth - steps
		return steps > 0 && (landing == 0 || (landing == 1 && rest == 0))
	})
}

func depthBelowTheModuleRoot(dir string) int {
	if dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

func climb(call *ast.CallExpr) (steps, rest int) {
	if isStringOperation(call) {
		return 0, 0
	}
	isJoin := calls(call, "Join")
	args := call.Args
	if isJoin && len(args) > 0 && isOwnDirectory(args[0]) {
		args = args[1:]
	}
	var parts []string
	for _, arg := range args {
		text := stringLiteral(arg)
		switch {
		case text != nil && *text != "":
			parts = append(parts, *text)
		case text == nil && isJoin && len(parts) == 0:
			return 0, 0
		case text == nil && isJoin:
			parts = append(parts, "*")
		}
	}
	segments := strings.Split(path.Clean(strings.Join(parts, "/")), "/")
	for steps < len(segments) && segments[steps] == ".." {
		steps++
	}
	return steps, len(segments) - steps
}

func calls(call *ast.CallExpr, name string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == name
}

func isStringOperation(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && (pkg.Name == "strings" || pkg.Name == "bytes")
}

func isOwnDirectory(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	return ok && calls(call, "Dir")
}

func stringLiteral(expr ast.Expr) *string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return nil
	}
	text, err := strconv.Unquote(lit.Value)
	if err != nil {
		return nil
	}
	return &text
}

func walksUpToGoMod(node ast.Node) bool {
	return inspectFound(node, func(n ast.Node) bool {
		loop, ok := n.(*ast.ForStmt)
		return ok && loop.Init == nil && loop.Cond == nil && loop.Post == nil && namesGoMod(loop.Body)
	})
}

func namesGoMod(body *ast.BlockStmt) bool {
	return inspectFound(body, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		return ok && lit.Value == `"go.mod"`
	})
}

func inspectFound(node ast.Node, match func(ast.Node) bool) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		found = found || match(n)
		return !found
	})
	return found
}

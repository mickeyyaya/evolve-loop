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

	"github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet"
)

const awaitsTestLevelSelection = "a large package whose seam or single-writer tests read the tree; running the whole package at every ship is too slow, so it waits for test-level selection (inbox repo-contract-test-level-selection)"

var readsTheTreeOutsideThePack = map[string]string{
	"cmd/evolve":                    awaitsTestLevelSelection,
	"internal/bridge":               awaitsTestLevelSelection,
	"internal/changedpkgs":          awaitsTestLevelSelection,
	"internal/core":                 awaitsTestLevelSelection,
	"internal/cycleoutcome":         awaitsTestLevelSelection,
	"internal/inboxmover":           awaitsTestLevelSelection,
	"internal/inboxmover/lifecycle": awaitsTestLevelSelection,
	"internal/phaseobserver":        awaitsTestLevelSelection,
	"internal/phases/audit":         awaitsTestLevelSelection,
	"internal/phases/runner":        awaitsTestLevelSelection,
	"internal/phases/ship":          awaitsTestLevelSelection,
	"internal/reachabilityprobe":    awaitsTestLevelSelection,
	"internal/subagent":             awaitsTestLevelSelection,
}

func TestPackages_HoldEveryTestThatReadsTheWholeTree(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	files, _, err := rawgitratchet.BoundTestFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	readers := treeReaders(t, root, files)
	for _, want := range []string{"internal/repocontract", "internal/sizeratchet", "internal/policy", "internal/acssuite", "internal/guards"} {
		if !slices.Contains(readers, want) {
			t.Fatalf("the detector must find %s; it found %v", want, readers)
		}
	}
	for _, problem := range packProblems(readers, Packages(), readsTheTreeOutsideThePack) {
		t.Error(problem)
	}
}

func packProblems(readers, pack []string, outside map[string]string) []string {
	inPack := map[string]bool{}
	for _, pattern := range pack {
		inPack[strings.TrimSuffix(strings.TrimPrefix(pattern, "./"), "/...")] = true
	}
	var problems []string
	for _, dir := range readers {
		if !inPack[dir] && outside[dir] == "" {
			problems = append(problems, fmt.Sprintf("%s has a test that reads the whole tree, so a lane that touches neither it nor an importer can still break it; put it in Packages() or record why it is outside", dir))
		}
	}
	for _, dir := range slices.Sorted(maps.Keys(outside)) {
		if inPack[dir] || !slices.Contains(readers, dir) {
			problems = append(problems, fmt.Sprintf("%s is recorded outside the pack but is in it or no longer reads the tree; remove the record", dir))
		}
	}
	return problems
}

func treeReaders(t *testing.T, root string, files []string) []string {
	t.Helper()
	var dirs []string
	for _, rel := range files {
		dir := path.Dir(rel)
		if strings.HasPrefix(rel, "acs/") || slices.Contains(dirs, dir) {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, filepath.FromSlash(rel)), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		if climbsOutOfItsPackage(file, dir) || walksUpToGoMod(file) {
			dirs = append(dirs, dir)
		}
	}
	slices.Sort(dirs)
	return dirs
}

func climbsOutOfItsPackage(file *ast.File, dir string) bool {
	depth := depthBelowTheModuleRoot(dir)
	return inspectFound(file, func(n ast.Node) bool {
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

func walksUpToGoMod(file *ast.File) bool {
	return inspectFound(file, func(n ast.Node) bool {
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

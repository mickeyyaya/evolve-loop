package structure

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var fenceProjections = []string{"FenceFor", "fenceFor"}

type phaseRequestBuilder struct {
	at     string
	fenced bool
}

func phaseRequestBuilders(root string, dirs []string) ([]phaseRequestBuilder, error) {
	files, err := SourceFiles(root, dirs...)
	if err != nil {
		return nil, err
	}
	var out []phaseRequestBuilder
	for _, rel := range files {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(root, rel), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok || !isPhaseRequestType(lit.Type) || keyValue(lit, "Worktree") == nil {
				return true
			}
			out = append(out, phaseRequestBuilder{at: fmt.Sprintf("%s:%d", rel, fset.Position(lit.Pos()).Line), fenced: isProjectionCall(keyValue(lit, "Fence"))})
			return true
		})
	}
	return out, nil
}

func isPhaseRequestType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name == "PhaseRequest"
	case *ast.SelectorExpr:
		return t.Sel.Name == "PhaseRequest"
	}
	return false
}

func keyValue(lit *ast.CompositeLit, key string) ast.Expr {
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if id, ok := kv.Key.(*ast.Ident); ok && id.Name == key {
				return kv.Value
			}
		}
	}
	return nil
}

func isProjectionCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return slices.Contains(fenceProjections, fn.Name)
	case *ast.SelectorExpr:
		return slices.Contains(fenceProjections, fn.Sel.Name)
	}
	return false
}

func TestEveryPhaseRequestWithAWorktreeTakesTheFenceFromTheOneProjection(t *testing.T) {
	t.Parallel()
	builders, err := phaseRequestBuilders(filepath.Join("..", ".."), []string{"internal", "cmd"})
	if err != nil {
		t.Fatalf("phaseRequestBuilders = %v", err)
	}
	var unfenced []string
	for _, b := range builders {
		if !b.fenced {
			unfenced = append(unfenced, b.at)
		}
	}
	if len(builders) < 8 {
		t.Fatalf("found %d PhaseRequest builder(s) with a worktree, want the 8 known roots at least: %v", len(builders), builders)
	}
	if len(unfenced) > 0 {
		t.Errorf("%d PhaseRequest builder(s) set a worktree without Fence from FenceFor or fenceFor:\n%s", len(unfenced), strings.Join(unfenced, "\n"))
	}
}

func TestPhaseRequestBuilders_TellAProjectedFenceFromAMissingOrHandBuiltOne(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/x/x.go": "package x\nfunc f() {\n" +
			"\t_ = core.PhaseRequest{Worktree: w, Fence: core.FenceFor(false, nil)}\n" +
			"\t_ = PhaseRequest{Worktree: w, Fence: o.fenceFor(p, cs)}\n" +
			"\t_ = core.PhaseRequest{Worktree: w}\n" +
			"\t_ = PhaseRequest{Worktree: w, Fence: &WorktreeFence{}}\n" +
			"\t_ = PhaseRequest{Workspace: ws}\n" +
			"\t_ = Other{Worktree: w}\n}\n",
	})

	builders, err := phaseRequestBuilders(root, []string{"internal"})

	got := make([]bool, len(builders))
	for i, b := range builders {
		got[i] = b.fenced
	}
	if err != nil || !slices.Equal(got, []bool{true, true, false, false}) {
		t.Fatalf("builders = %+v, %v; want two projected, one missing and one hand-built fence, and no literal without a worktree", builders, err)
	}
}

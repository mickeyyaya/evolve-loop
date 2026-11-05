package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

const hostEffectsCycle = 4

func hostEffectsWorkspace(t *testing.T, withRecord bool) (string, core.ReviewInput, *deliverable.HostEffects) {
	t.Helper()
	catalog, err := phasespec.Load(filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	items := map[string]string{
		"2026-01-01T00-00-00Z-x.json": `{"id":"x","title":"fixture","weight":0.9,"files":["go/internal/guards/role.go"]}`,
		"2026-01-01T00-00-01Z-y.json": `{"id":"y","title":"fixture","weight":0.8}`,
	}
	for name, body := range items {
		if err := os.WriteFile(filepath.Join(inbox, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ws := filepath.Join(root, ".evolve", "runs", "cycle-4")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if withRecord {
		if err := committedset.WriteRecord(root, committedset.Record{Cycle: hostEffectsCycle, RunID: "r", ItemIDs: []string{"x", "y"}, GoalHash: "g"}); err != nil {
			t.Fatal(err)
		}
	}
	decision := `{"cycle":4,"top_n":[{"id":"x"},{"id":"y"}],"deferred":[]}`
	report := "# Triage Decision\n\n## top_n\n- x: fixture — priority=H, source={scout}\n- y: fixture — priority=H, source={scout}\n"
	for name, body := range map[string]string{"triage-decision.json": decision, "triage-report.md": report} {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	forbidden := func(p string) bool { return p == "go/internal/guards/role.go" }
	in := core.ReviewInput{Cycle: hostEffectsCycle, Phase: "triage", Workspace: ws, ProjectRoot: root}
	return root, in, newHostEffects(catalog, nil, nil, forbidden)
}

func TestNewHostEffects_AProtectedFilesItemIsRoutedAndAClaimableItemIsClaimed(t *testing.T) {
	t.Parallel()
	root, in, fx := hostEffectsWorkspace(t, true)
	ws := in.Workspace
	if err := fx.Perform(context.Background(), in); err != nil {
		t.Fatalf("host effects: %v", err)
	}
	if got := committedset.RoutedIDs(ws); !slices.Equal(got, []string{"x"}) {
		t.Fatalf("the composition root must route the protected-files item; routed=%v", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox", "processing", "cycle-4", "2026-01-01T00-00-01Z-y.json")); err != nil {
		t.Fatalf("the claimable item must be claimed into processing/cycle-4: %v", err)
	}
}

func TestNewHostEffects_AFailedRouteLeavesAProtectedFilesItemAtTheInboxRoot(t *testing.T) {
	t.Parallel()
	root, in, fx := hostEffectsWorkspace(t, false)
	if err := fx.Perform(context.Background(), in); err == nil {
		t.Fatalf("host effects = nil, want the failed route")
	}
	name := "2026-01-01T00-00-00Z-x.json"
	if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox", name)); err != nil {
		t.Errorf("the claimer must refuse the protected-files item, so it stays at the inbox root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox", "processing", "cycle-4", name)); err == nil {
		t.Errorf("the protected-files item was claimed into processing/cycle-4")
	}
}

func TestWireOrchestratorDeps_PassesTheForbiddenPredicateToTheHostEffects(t *testing.T) {
	t.Parallel()
	file, err := parser.ParseFile(token.NewFileSet(), "cmd_cycle.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var args []ast.Expr
	var assigned string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "wireOrchestratorDeps" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				if id, isIdent := node.Fun.(*ast.Ident); isIdent && id.Name == "newHostEffects" {
					args = node.Args
				}
			case *ast.AssignStmt:
				if id, isIdent := node.Lhs[0].(*ast.Ident); isIdent && id.Name == "forbidden" {
					if call, isCall := node.Rhs[0].(*ast.CallExpr); isCall {
						if builder, isName := call.Fun.(*ast.Ident); isName {
							assigned = builder.Name
						}
					}
				}
			}
			return true
		})
	}
	if len(args) != 4 {
		t.Fatalf("newHostEffects call args = %d, want 4", len(args))
	}
	last, ok := args[3].(*ast.Ident)
	if !ok || last.Name != "forbidden" || assigned != "laneForbiddenTo" {
		t.Errorf("predicate = %v assigned from %q, want forbidden from laneForbiddenTo", args[3], assigned)
	}
}

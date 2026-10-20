package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestWireOrchestrator_TheNoWorkCheckAsksTheLaneMenu(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if d := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{}); !d.Orchestrator.LaneMenuWired() {
		t.Fatal("the production composition root must give the no-work check the lane menu triage offers, or a sequential cycle with only console-owned or waiting work seals a claim failure")
	}
}

func TestWireSimulateOrchestrator_TheNoWorkCheckAsksTheLaneMenu(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	d := wireSimulateOrchestrator(root, evolveDir, io.Discard)
	defer d.CloseSignals()
	if !d.Orchestrator.LaneMenuWired() {
		t.Fatal("the simulate composition root must give the no-work check the lane menu, as the cycle root does")
	}
}

func TestCompositionRoots_EveryOrchestratorRootWiresTheLaneMenu(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	roots := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			builds, wires := false, false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if sel, isSel := n.(*ast.SelectorExpr); isSel {
					if pkg, isIdent := sel.X.(*ast.Ident); isIdent && pkg.Name == "core" {
						builds = builds || sel.Sel.Name == "NewOrchestrator"
						wires = wires || sel.Sel.Name == "WithLaneMenu"
					}
				}
				return true
			})
			if builds {
				roots++
				if !wires {
					t.Errorf("%s: %s builds an orchestrator without core.WithLaneMenu; only the routingtest root may leave the lane menu nil", name, fn.Name.Name)
				}
			}
		}
	}
	if roots < 2 {
		t.Fatalf("found %d orchestrator roots, want the cycle and simulate roots at least", roots)
	}
}

func TestLaneMenuOf_WithholdsProtectedAndWaitingItems(t *testing.T) {
	t.Parallel()
	inboxRoot := filepath.Join(t.TempDir(), ".evolve", "inbox")
	if err := os.MkdirAll(inboxRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inboxRoot, "blocker.json"), []byte(`{"id":"blocker"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	guarded := func(path string) bool { return path == "go/internal/guarded/x.go" }
	items := []inboxbatch.Item{
		{ID: "blocker"},
		{ID: "guarded", Files: []string{"go/internal/guarded/x.go"}},
		{ID: "waiting", Deps: []string{"blocker"}},
	}
	ready := laneMenuOf(guarded)(inboxRoot, items)
	if len(ready) != 1 || ready[0].ID != "blocker" {
		t.Errorf("the lane menu offers only the ready item: %+v", ready)
	}
}

func TestLaneMenuOf_TheProductionPredicateWithholdsARootThatHoldsOnlyAProtectedFileItem(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inboxRoot := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inboxRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	menu := laneMenuOf(laneForbidden(root, io.Discard))
	items := []inboxbatch.Item{{ID: "protected-only", Files: []string{"go/internal/core/orchestrator.go"}}}
	if ready := menu(inboxRoot, items); len(ready) != 0 {
		t.Fatalf("a root whose only item declares a protected file offers no lane work, so the no-work check ends the cycle as planned no-work: %+v", ready)
	}
	if ready := menu(inboxRoot, []inboxbatch.Item{{ID: "lane-work", Files: []string{"go/internal/fleet/fleet.go"}}}); len(ready) != 1 {
		t.Fatalf("ordinary lane work stays on the menu: %+v", ready)
	}
}

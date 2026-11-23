package stageevidence_test

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

const stageEvidencePkgPath = "github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"

var routedGuardRoot = flag.String("routed-guard-root", "", "Go directory root scanned for stageevidence routed-item emit sites")

type scanTarget struct {
	dir   string
	label string
}

func TestStageEvidenceRoutedSourceGuard(t *testing.T) {
	verifyConflictClassReachable(t)
	verifySealOutcomeUnrouted(t)
	targets := defaultGuardTargets()
	if root := *routedGuardRoot; root != "" {
		targets = []scanTarget{{dir: root}}
	}
	findings := scanRoutedEmitSites(t, targets)
	if len(findings) > 0 {
		t.Errorf("stageevidence routed-item emit site detected while pending F-item F4 conflict-class coverage is unverified:\n  %s\nAdd a scorer test reaching ClassConflict through each emit site and register it.", strings.Join(findings, "\n  "))
		return
	}
	t.Logf("stageevidence routed-source guard: 0 emit sites fill Outcome.Routed or held-proposal item (pending F-item F4)")
}

func verifyConflictClassReachable(t *testing.T) {
	t.Helper()
	rec := stageevidence.Link("", stageevidence.Record{
		Key:     "held_proposals.stage",
		Version: "v1",
		Kind:    stageevidence.KindDecision,
		ID:      1,
		Seq:     1,
		Fields:  map[string]string{"would_hold": "true", "item": "item-1"},
	})
	chain := stageevidence.Chain{
		Records:  []stageevidence.Record{rec},
		Outcomes: []stageevidence.Outcome{{ID: 1, Routed: []string{"item-1"}}},
	}
	res := stageevidence.Score(chain, "v1", gatecontract.Promotion{MinDecisions: 1}, stageevidence.DefaultScorer())
	if res.Conflicts != 1 || res.Ready {
		t.Fatalf("stageevidence conflict class unreachable: conflicts=%d ready=%v", res.Conflicts, res.Ready)
	}
}

func verifySealOutcomeUnrouted(t *testing.T) {
	t.Helper()
	seal := stageevidence.Seal{V: stageevidence.SealVersion, Count: 1, Last: "h", Shipped: true}
	if out := seal.Outcome(stageevidence.CycleScope(1)); len(out.Routed) != 0 {
		t.Fatalf("stageevidence.Seal.Outcome unexpectedly filled Routed (%v) while pending F-item F4 conflict-class coverage is unverified", out.Routed)
	}
}

func defaultGuardTargets() []scanTarget {
	return []scanTarget{
		{dir: ".", label: "internal/stageevidence"},
		{dir: filepath.Join("..", "adapters", "ledger"), label: "internal/adapters/ledger"},
		{dir: filepath.Join("..", "heldproposal"), label: "internal/heldproposal"},
		{dir: filepath.Join("..", "phases", "ship"), label: "internal/phases/ship"},
		{dir: filepath.Join("..", "core"), label: "internal/core"},
		{dir: filepath.Join("..", "orchestrator"), label: "internal/orchestrator"},
		{dir: filepath.Join("..", "gateregistry"), label: "internal/gateregistry"},
	}
}

func scanRoutedEmitSites(t *testing.T, targets []scanTarget) []string {
	t.Helper()
	fset := token.NewFileSet()
	var findings []string
	for _, target := range targets {
		hits, err := walkRoutedTarget(fset, target)
		if err != nil {
			t.Fatalf("scan routed emit sites under %s: %v", target.dir, err)
		}
		findings = append(findings, hits...)
	}
	return findings
}

func walkRoutedTarget(fset *token.FileSet, target scanTarget) ([]string, error) {
	var findings []string
	err := filepath.WalkDir(target.dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if name := d.Name(); name == ".git" || name == "testdata" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		hits, err := inspectRoutedSourceFile(fset, path)
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(target.dir, path)
		if relErr != nil {
			rel = path
		}
		if target.label != "" {
			rel = filepath.Join(target.label, rel)
		}
		for _, hit := range hits {
			findings = append(findings, fmt.Sprintf("%s: %s", filepath.ToSlash(rel), hit))
		}
		return nil
	})
	return findings, err
}

func inspectRoutedSourceFile(fset *token.FileSet, path string) ([]string, error) {
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	aliases, dotImport, inPkg := stageEvidenceImports(file)
	hasStageRef := inPkg || dotImport || len(aliases) > 0
	mentionsWouldHold := fileHasStringLit(file, "would_hold")
	var hits []string
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
			nonOut := nonOutcomeVars(fn, aliases, dotImport, inPkg)
			hits = append(hits, inspectASTBlock(fset, fn.Body, aliases, dotImport, inPkg, hasStageRef, mentionsWouldHold, nonOut)...)
			continue
		}
		hits = append(hits, inspectASTBlock(fset, decl, aliases, dotImport, inPkg, hasStageRef, mentionsWouldHold, nil)...)
	}
	return hits, nil
}

func stageEvidenceImports(file *ast.File) (map[string]bool, bool, bool) {
	aliases := map[string]bool{}
	dotImport := false
	for _, imp := range file.Imports {
		val, err := strconv.Unquote(imp.Path.Value)
		if err != nil || val != stageEvidencePkgPath {
			continue
		}
		if imp.Name == nil {
			aliases["stageevidence"] = true
			continue
		}
		if imp.Name.Name == "." {
			dotImport = true
			continue
		}
		if imp.Name.Name != "_" {
			aliases[imp.Name.Name] = true
		}
	}
	return aliases, dotImport, file.Name.Name == "stageevidence"
}

func inspectASTBlock(fset *token.FileSet, root ast.Node, aliases map[string]bool, dotImport, inPkg, hasStageRef, mentionsWouldHold bool, nonOut map[string]bool) []string {
	var hits []string
	ast.Inspect(root, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CompositeLit:
			if hasStageRef && isOutcomeType(x.Type, aliases, dotImport, inPkg) && hasKeyIdent(x.Elts, "Routed") {
				hits = append(hits, fmt.Sprintf("line %d: Outcome literal fills Routed", fset.Position(x.Pos()).Line))
			}
			if hasStageRef && isOutcomeSliceType(x.Type, aliases, dotImport, inPkg) {
				hits = append(hits, sliceOutcomeRoutedHits(fset, x.Elts)...)
			}
			if hasMapKeyLit(x.Elts, "would_hold") && hasMapKeyLit(x.Elts, "item") {
				hits = append(hits, fmt.Sprintf("line %d: held-proposal fields literal carries item", fset.Position(x.Pos()).Line))
			}
		case *ast.AssignStmt:
			hits = append(hits, inspectAssignHits(fset, x, hasStageRef, mentionsWouldHold, nonOut)...)
		}
		return true
	})
	return hits
}

func sliceOutcomeRoutedHits(fset *token.FileSet, elts []ast.Expr) []string {
	var hits []string
	for _, elt := range elts {
		lit, ok := elt.(*ast.CompositeLit)
		if ok && hasKeyIdent(lit.Elts, "Routed") {
			hits = append(hits, fmt.Sprintf("line %d: Outcome literal fills Routed", fset.Position(lit.Pos()).Line))
		}
	}
	return hits
}

func inspectAssignHits(fset *token.FileSet, stmt *ast.AssignStmt, hasStageRef, mentionsWouldHold bool, nonOut map[string]bool) []string {
	var hits []string
	for _, lhs := range stmt.Lhs {
		expr := unparenExpr(lhs)
		if sel, ok := expr.(*ast.SelectorExpr); ok && sel.Sel.Name == "Routed" {
			if hasStageRef && !isKnownNonOutcome(sel.X, nonOut) {
				hits = append(hits, fmt.Sprintf("line %d: assignment fills Outcome.Routed", fset.Position(stmt.Pos()).Line))
			}
		}
		if ix, ok := expr.(*ast.IndexExpr); ok && stringLitValue(ix.Index) == "item" {
			if (hasStageRef || mentionsWouldHold) && (isFieldsExpr(ix.X) || mentionsWouldHold) {
				hits = append(hits, fmt.Sprintf("line %d: assignment fills held-proposal item", fset.Position(stmt.Pos()).Line))
			}
		}
	}
	return hits
}

func isOutcomeType(expr ast.Expr, aliases map[string]bool, dotImport, inPkg bool) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return (inPkg || dotImport) && t.Name == "Outcome"
	case *ast.SelectorExpr:
		id, ok := t.X.(*ast.Ident)
		return ok && aliases[id.Name] && t.Sel.Name == "Outcome"
	}
	return false
}

func isOutcomeSliceType(expr ast.Expr, aliases map[string]bool, dotImport, inPkg bool) bool {
	arr, ok := expr.(*ast.ArrayType)
	return ok && isOutcomeType(arr.Elt, aliases, dotImport, inPkg)
}

func nonOutcomeVars(fn *ast.FuncDecl, aliases map[string]bool, dotImport, inPkg bool) map[string]bool {
	nonOut := map[string]bool{}
	recordFieldList := func(fl *ast.FieldList) {
		if fl == nil {
			return
		}
		for _, field := range fl.List {
			if !isOutcomeType(field.Type, aliases, dotImport, inPkg) {
				for _, name := range field.Names {
					nonOut[name.Name] = true
				}
			}
		}
	}
	recordFieldList(fn.Type.Params)
	recordFieldList(fn.Type.Results)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ValueSpec:
			if x.Type != nil && !isOutcomeType(x.Type, aliases, dotImport, inPkg) {
				for _, name := range x.Names {
					nonOut[name.Name] = true
				}
			}
		case *ast.AssignStmt:
			if x.Tok == token.DEFINE && len(x.Lhs) == len(x.Rhs) {
				for i, rhs := range x.Rhs {
					if lit, ok := rhs.(*ast.CompositeLit); ok && lit.Type != nil && !isOutcomeType(lit.Type, aliases, dotImport, inPkg) {
						if id, ok := x.Lhs[i].(*ast.Ident); ok {
							nonOut[id.Name] = true
						}
					}
				}
			}
		}
		return true
	})
	return nonOut
}

func isKnownNonOutcome(expr ast.Expr, nonOut map[string]bool) bool {
	id, ok := unparenExpr(expr).(*ast.Ident)
	return ok && nonOut[id.Name]
}

func isFieldsExpr(expr ast.Expr) bool {
	switch x := unparenExpr(expr).(type) {
	case *ast.SelectorExpr:
		return x.Sel.Name == "Fields"
	case *ast.Ident:
		return x.Name == "fields" || x.Name == "Fields"
	}
	return false
}

func hasKeyIdent(elts []ast.Expr, key string) bool {
	for _, elt := range elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if id, ok := kv.Key.(*ast.Ident); ok && id.Name == key {
			return true
		}
	}
	return false
}

func hasMapKeyLit(elts []ast.Expr, key string) bool {
	for _, elt := range elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if ok && stringLitValue(kv.Key) == key {
			return true
		}
	}
	return false
}

func fileHasStringLit(file *ast.File, want string) bool {
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && stringLitValue(lit) == want {
			found = true
			return false
		}
		return !found
	})
	return found
}

func stringLitValue(expr ast.Expr) string {
	lit, ok := unparenExpr(expr).(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	val, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return val
}

func unparenExpr(expr ast.Expr) ast.Expr {
	for {
		p, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = p.X
	}
}

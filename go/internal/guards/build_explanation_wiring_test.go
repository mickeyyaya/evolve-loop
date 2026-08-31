package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

func TestBuildExplanationLifecycleWiring(t *testing.T) {
	pins := []struct {
		path     string
		function string
		callee   string
	}{
		{"../phases/ship/native.go", "Run", "verifyNativeExplanation"},
		{"../phases/audit/audit.go", "Classify", "validateExplanationReview"},
		{"../phases/retro/retro.go", "Run", "validateExplanationReview"},
		{"../core/cyclerun_review.go", "reviewAndGuard", "explanationdocs.RefreshResult"},
		{"../core/orchestrator.go", "RunCycle", "activateBuildExplanationContract"},
		{"../core/orchestrator.go", "RunCycle", "sealBuildExplanationContext"},
		{"../core/cyclerun_dispatch.go", "dispatch", "projectBuildExplanation"},
		{"../core/resume.go", "RunCycleFromPhase", "explanationdocs.RefreshResult"},
		{"../core/resume.go", "RunCycleFromPhase", "sealBuildExplanationContext"},
		{"../core/resume.go", "RunCycleFromPhase", "projectBuildExplanation"},
	}
	for _, pin := range pins {
		t.Run(filepath.Base(pin.path)+"/"+pin.function, func(t *testing.T) {
			if !functionCalls(t, pin.path, pin.function, pin.callee) {
				t.Fatalf("%s must call %s", pin.function, pin.callee)
			}
		})
	}
	assignments := []struct {
		path, function, field, value string
	}{
		{"../core/orchestrator.go", "NewOrchestrator", "explanationContractVersion", "explanationdocs.CurrentContractVersion"},
		{"../core/cyclerun.go", "newCycleRun", "ExplanationDocumentationVersion", "o.explanationContractVersion"},
		{"../phases/runner/runner.go", "Run", "RequireSandbox", "requiresExplanationSandbox"},
	}
	for _, pin := range assignments {
		t.Run(filepath.Base(pin.path)+"/"+pin.field, func(t *testing.T) {
			if !functionAssigns(t, pin.path, pin.function, pin.field, pin.value) {
				t.Fatalf("%s must assign %s from %s", pin.function, pin.field, pin.value)
			}
		})
	}
}

func functionCalls(t *testing.T, path, function, callee string) bool {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || fn.Name.Name != function || fn.Body == nil {
			continue
		}
		found := false
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if ok && expressionName(call.Fun) == callee {
				found = true
				return false
			}
			return !found
		})
		return found
	}
	return false
}

func expressionName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		prefix := expressionName(value.X)
		if prefix == "" {
			return value.Sel.Name
		}
		return prefix + "." + value.Sel.Name
	case *ast.CallExpr:
		return expressionName(value.Fun)
	default:
		return ""
	}
}

func functionAssigns(t *testing.T, path, function, field, value string) bool {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || fn.Name.Name != function || fn.Body == nil {
			continue
		}
		found := false
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			element, ok := node.(*ast.KeyValueExpr)
			key, keyOK := elementKey(element)
			if ok && keyOK && key == field && expressionName(element.Value) == value {
				found = true
				return false
			}
			return !found
		})
		return found
	}
	return false
}

func elementKey(element *ast.KeyValueExpr) (string, bool) {
	if element == nil {
		return "", false
	}
	key, ok := element.Key.(*ast.Ident)
	if !ok {
		return "", false
	}
	return key.Name, true
}

package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// explanationLifecycleCallPins is the SINGLE list of "this function must call
// that explanation-lifecycle callee" pins. It is package-level (not a local
// literal) so integrity_surface_explanation_callsites_test.go can project the
// SAME belief into a call-site scanner's vocabulary — a pin that lives only
// inside one test function's body cannot be read by another test.
var explanationLifecycleCallPins = []struct {
	path     string
	function string
	callee   string
}{
	{"../phases/ship/native.go", "Run", "verifyNativeExplanation"},
	{"../phases/audit/classification.go", "newAuditClassification", "validateExplanationReview"},
	{"../phases/retro/retro.go", "Run", "validateExplanationReview"},
	{"../core/cyclerun_postreview.go", "applyPostReviewGuards", "explanationdocs.RefreshResult"},
	{"../core/orchestrator.go", "RunCycle", "activateBuildExplanationContract"},
	{"../core/orchestrator.go", "RunCycle", "sealBuildExplanationContext"},
	{"../core/cyclerun_dispatch.go", "dispatch", "projectBuildExplanation"},
	{"../core/resume_execution.go", "run", "explanationdocs.RefreshResult"},
	{"../core/resume_execution.go", "run", "sealBuildExplanationContext"},
	{"../core/resume_execution.go", "run", "projectBuildExplanation"},
}

// explanationLifecycleAssignPins is the companion list for a callee that
// reaches the lifecycle by VALUE (a struct-literal field assignment) rather
// than a bare call. expressionName still resolves a wrapped call expression
// (e.g. `RequireSandbox: requiresExplanationSandbox(phase, req)`) to the
// called function's name, so this list also feeds the call-site vocabulary.
var explanationLifecycleAssignPins = []struct {
	path, function, field, value string
}{
	{"../core/orchestrator.go", "NewOrchestrator", "explanationContractVersion", "explanationdocs.CurrentContractVersion"},
	{"../core/cyclerun.go", "newCycleRun", "ExplanationDocumentationVersion", "o.explanationContractVersion"},
	{"../phases/runner/dispatch.go", "dispatchPhaseAttempts", "RequireSandbox", "requiresExplanationSandbox"},
}

func TestBuildExplanationLifecycleWiring(t *testing.T) {
	for _, pin := range explanationLifecycleCallPins {
		t.Run(filepath.Base(pin.path)+"/"+pin.function, func(t *testing.T) {
			if !functionCalls(t, pin.path, pin.function, pin.callee) {
				t.Fatalf("%s must call %s", pin.function, pin.callee)
			}
		})
	}
	for _, pin := range explanationLifecycleAssignPins {
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

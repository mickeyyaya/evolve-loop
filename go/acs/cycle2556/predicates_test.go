//go:build acs

package cycle2556

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	observerPkg       = "internal/phaseobserver"
	toolUsingTestName = "TestRun_MaxNoProgress_ToolUsingAgent_DoesNotFire"
	eventsFileName    = "builder-observer-events.ndjson"
)

func TestC2556_001_ToolUsingTestFailsWhenEventsFileIsMissing(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	testPath := filepath.Join(goDir, observerPkg, "maxnoprogress_test.go")
	src, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("read %s: %v", testPath, err)
	}
	if !strings.Contains(string(src), eventsFileName) {
		t.Fatalf("%s no longer names %s; the mutation cannot point the test at a missing file", testPath, eventsFileName)
	}
	eventsFileAbsent := strings.ReplaceAll(string(src), eventsFileName, "builder-observer-absent.ndjson")

	tmp := t.TempDir()
	mutated := filepath.Join(tmp, "maxnoprogress_test.go")
	if err := os.WriteFile(mutated, []byte(eventsFileAbsent), 0o644); err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(map[string]map[string]string{"Replace": {testPath: mutated}})
	if err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(tmp, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1",
		"-overlay", overlayPath, "-run", "^"+toolUsingTestName+"$", "./"+observerPkg)
	out := stdout + stderr
	if code == 0 {
		t.Errorf("RED: %s passes although its events file is missing:\n%s", toolUsingTestName, acsassert.OutputTail(out, 15))
	}
	if code != 0 && !strings.Contains(out, "--- FAIL: "+toolUsingTestName) {
		t.Errorf("mutated run failed without failing %s (build error?):\n%s", toolUsingTestName, acsassert.OutputTail(out, 15))
	}
}

func ignoredUnmarshalLines(t *testing.T, filename, src string) []int {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}
	isUnmarshal := func(e ast.Expr) bool {
		call, ok := e.(*ast.CallExpr)
		if !ok {
			return false
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Unmarshal" {
			return false
		}
		pkg, ok := sel.X.(*ast.Ident)
		return ok && pkg.Name == "json"
	}
	var lines []int
	ast.Inspect(file, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.ExprStmt:
			if isUnmarshal(s.X) {
				lines = append(lines, fset.Position(s.Pos()).Line)
			}
		case *ast.AssignStmt:
			if len(s.Rhs) == 1 && isUnmarshal(s.Rhs[0]) {
				if id, ok := s.Lhs[len(s.Lhs)-1].(*ast.Ident); ok && id.Name == "_" {
					lines = append(lines, fset.Position(s.Pos()).Line)
				}
			}
		}
		return true
	})
	return lines
}

func TestC2556_002_DetectorFlagsIgnoredUnmarshalAndSparesCheckedOnes(t *testing.T) {
	ignored := "package p\nimport \"encoding/json\"\nfunc f(b []byte) {\n\tvar d map[string]any\n\t_ = json.Unmarshal(b, &d)\n\tjson.Unmarshal(b, &d)\n}\n"
	if got := ignoredUnmarshalLines(t, "ignored.go", ignored); len(got) != 2 {
		t.Errorf("detector must flag the blank-assigned and the bare call, got lines %v", got)
	}
	checked := "package p\nimport \"encoding/json\"\nfunc f(b []byte) error {\n\tvar d map[string]any\n\tif err := json.Unmarshal(b, &d); err != nil {\n\t\treturn err\n\t}\n\treturn json.Unmarshal(b, &d)\n}\n"
	if got := ignoredUnmarshalLines(t, "checked.go", checked); len(got) != 0 {
		t.Errorf("detector must spare checked calls, got lines %v", got)
	}
}

func TestC2556_003_NoPhaseobserverTestIgnoresAnUnmarshalError(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	files, err := filepath.Glob(filepath.Join(goDir, observerPkg, "*_test.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no phaseobserver test files found: %v", err)
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, line := range ignoredUnmarshalLines(t, f, string(src)) {
			t.Errorf("RED: %s:%d ignores a json.Unmarshal error", filepath.Base(f), line)
		}
	}
}

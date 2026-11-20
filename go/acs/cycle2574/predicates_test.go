//go:build acs

package cycle2574

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const stopTestName = "TestRun_StopSignalEndsRunAndClosesFile"

func loadStopTest(t *testing.T) *ast.FuncDecl {
	t.Helper()
	path := filepath.Join(acsassert.GoModuleDir(t), "cmd", "hostsampler", "main_test.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == stopTestName {
			return fn
		}
	}
	t.Fatalf("%s not found in %s", stopTestName, path)
	return nil
}

func isPkgCall(call *ast.CallExpr, pkg, name string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg
}

func isIdentCall(call *ast.CallExpr, name string) bool {
	id, ok := call.Fun.(*ast.Ident)
	return ok && id.Name == name
}

func TestC2574_001_StopTestHasNoSleep(t *testing.T) {
	fn := loadStopTest(t)
	ast.Inspect(fn, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && isPkgCall(call, "time", "Sleep") {
			t.Errorf("%s still calls time.Sleep", stopTestName)
		}
		return true
	})
}

func TestC2574_002_StopTestStartsNoGoroutineToSendTheStop(t *testing.T) {
	fn := loadStopTest(t)
	ast.Inspect(fn, func(n ast.Node) bool {
		if _, ok := n.(*ast.GoStmt); ok {
			t.Errorf("%s still starts a goroutine; the stop must come from the reader", stopTestName)
		}
		return true
	})
}

func TestC2574_003_StopIsSentFromTheReaderAfterACountedRead(t *testing.T) {
	fn := loadStopTest(t)
	sendInCountedReader := false
	ast.Inspect(fn, func(n ast.Node) bool {
		lit, ok := n.(*ast.FuncLit)
		if !ok || lit.Type.Results == nil || len(lit.Type.Results.List) != 2 {
			return true
		}
		sends, counts := false, false
		ast.Inspect(lit.Body, func(m ast.Node) bool {
			switch v := m.(type) {
			case *ast.SendStmt:
				sends = true
			case *ast.IfStmt:
				if _, isCmp := v.Cond.(*ast.BinaryExpr); isCmp {
					counts = true
				}
			}
			return true
		})
		if sends && counts {
			sendInCountedReader = true
		}
		return true
	})
	if !sendInCountedReader {
		t.Errorf("%s has no reader func literal that sends on the stop channel under a counted condition", stopTestName)
	}
}

func TestC2574_004_StopTestStillAssertsExitCodeAndSamples(t *testing.T) {
	fn := loadStopTest(t)
	runs, readsSamples, failsOnSamples := 0, 0, false
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch {
		case isIdentCall(call, "run"):
			runs++
		case isIdentCall(call, "readSamples"):
			readsSamples++
		case isPkgCall(call, "t", "Fatal") || isPkgCall(call, "t", "Fatalf") || isPkgCall(call, "t", "Errorf"):
			failsOnSamples = true
		}
		return true
	})
	if runs != 1 || readsSamples != 1 || !failsOnSamples {
		t.Errorf("stop test must call run once (%d), read samples once (%d) and fail on a bad outcome (%v)", runs, readsSamples, failsOnSamples)
	}
}

func TestC2574_005_StopTestPassesFiftyTimesUnderRace(t *testing.T) {
	cmd := exec.Command("go", "test", "-race", "-count=50", "-v", "./cmd/hostsampler/", "-run", "^"+stopTestName+"$")
	cmd.Dir = acsassert.GoModuleDir(t)
	raw, err := cmd.CombinedOutput()
	out := string(raw)
	if err != nil {
		t.Fatalf("go test -race -count=50 failed: %v\n%s", err, acsassert.OutputTail(out, 20))
	}
	passes := len(regexp.MustCompile(`(?m)^--- PASS: `+stopTestName).FindAllString(out, -1))
	if passes != 50 {
		t.Errorf("want 50 passing runs of %s, got %d", stopTestName, passes)
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("%s did not run", stopTestName)
	}
}

func TestC2574_006_SleepSyncBaselineRowForStopTestIsRetired(t *testing.T) {
	path := filepath.Join(acsassert.GoModuleDir(t), "cmd", "hostsampler", "testdata", "testaudit-baseline.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var shard struct {
		Rows []struct {
			Test string `json:"test"`
			Rule string `json:"rule"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(raw, &shard); err != nil {
		t.Fatal(err)
	}
	for _, row := range shard.Rows {
		if row.Test == stopTestName {
			t.Errorf("baseline still lists %s under rule %s", stopTestName, row.Rule)
		}
	}
}

func TestC2574_007_TestAuditRatchetStaysGreen(t *testing.T) {
	cmd := exec.Command("go", "test", "-count=1", "./internal/testaudit", "-run", "^TestTreeRatchet$")
	cmd.Dir = acsassert.GoModuleDir(t)
	if raw, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("testaudit ratchet failed: %v\n%s", err, acsassert.OutputTail(string(raw), 20))
	}
}

//go:build acs

package cycle2138

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const helperFile = "internal/gittest/credentials.go"

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runGo(t *testing.T, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = goDir(t)
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestC2138_001_ARealGhInATestSeesNoLogin(t *testing.T) {
	out, err := runGo(t, "test", "-count=1", "-v", "-run", "^TestIsolateHostCredentials_ARealGhInATestSeesNoLogin$", "./internal/gittest")
	if err != nil {
		t.Fatalf("go test: %v\n%s", err, acsassert.OutputTail(out, 30))
	}
	for _, sub := range []string{"hosts_file", "token_variable"} {
		if !strings.Contains(out, "--- PASS: TestIsolateHostCredentials_ARealGhInATestSeesNoLogin/"+sub) {
			t.Errorf("subtest %q did not pass (skipped or absent):\n%s", sub, acsassert.OutputTail(out, 30))
		}
	}
}

func TestC2138_002_TheHelperSetsHermeticValuesRestoresThemAndReportsFailures(t *testing.T) {
	out, err := runGo(t, "test", "-count=1", "-v", "-run", "^TestIsolateHostCredentials_", "./internal/gittest")
	if err != nil {
		t.Fatalf("go test: %v\n%s", err, acsassert.OutputTail(out, 30))
	}
	for _, name := range []string{
		"SetsTheHermeticValuesAndDropsHostTokens",
		"RestoreReinstatesThePriorEnvironment",
		"AConfigDirFailureIsReportedAndChangesNothing",
		"AFailedRemovalIsReportedAndTheEnvironmentIsStillRestored",
	} {
		if !strings.Contains(out, "--- PASS: TestIsolateHostCredentials_"+name) {
			t.Errorf("TestIsolateHostCredentials_%s did not pass", name)
		}
	}
}

func TestC2138_003_TheCmdEvolveTestMainCallsTheHelperAndChecksItsRestore(t *testing.T) {
	path := filepath.Join(goDir(t), "cmd", "evolve", "main_test.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse main_test.go: %v", err)
	}
	var testMain *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "TestMain" {
			testMain = fn
		}
	}
	if testMain == nil {
		t.Fatal("main_test.go has no TestMain")
	}
	var isolates, restores bool
	ast.Inspect(testMain.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Rhs) != 1 || len(assign.Lhs) != 2 {
			return true
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "IsolateHostCredentials" {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "gittest" {
			return true
		}
		isolates = true
		restoreName, _ := assign.Lhs[0].(*ast.Ident)
		ast.Inspect(testMain.Body, func(m ast.Node) bool {
			if c, ok := m.(*ast.CallExpr); ok && restoreName != nil {
				if id, ok := c.Fun.(*ast.Ident); ok && id.Name == restoreName.Name {
					restores = true
				}
			}
			return true
		})
		return true
	})
	if !isolates {
		t.Error("TestMain does not call gittest.IsolateHostCredentials")
	}
	if !restores {
		t.Error("TestMain never calls the restore function the helper returns")
	}
}

func TestC2138_004_EveryChangedFunctionHasFullStatementCoverage(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "cover.out")
	out, err := runGo(t, "test", "-count=1", "-coverprofile="+profile, "-run", "^TestIsolateHostCredentials_", "./internal/gittest")
	if err != nil {
		t.Fatalf("go test: %v\n%s", err, acsassert.OutputTail(out, 30))
	}
	fn, err := runGo(t, "tool", "cover", "-func="+profile)
	if err != nil {
		t.Fatalf("go tool cover: %v\n%s", err, fn)
	}
	pct, err := acsassert.FuncCoverage(fn, "IsolateHostCredentials")
	if err != nil {
		t.Fatalf("IsolateHostCredentials has no coverage line: %v", err)
	}
	if pct != 100 {
		t.Errorf("IsolateHostCredentials coverage = %.1f%%, want 100%%", pct)
	}
	inHelperFile := 0
	for _, line := range strings.Split(fn, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.Contains(fields[0], "internal/gittest/credentials.go:") {
			continue
		}
		inHelperFile++
		if fields[len(fields)-1] != "100.0%" {
			t.Errorf("function below 100%% statement coverage: %s", line)
		}
	}
	if inHelperFile == 0 {
		t.Errorf("no function of %s appears in the coverage report", helperFile)
	}
}

func TestC2138_005_TheHelperFileHasNoCommentAtAll(t *testing.T) {
	path := filepath.Join(goDir(t), filepath.FromSlash(helperFile))
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", helperFile, err)
	}
	if len(file.Comments) != 0 {
		t.Errorf("%s holds %d comment groups, want none", helperFile, len(file.Comments))
	}
	if file.Doc != nil {
		t.Errorf("%s has a package doc comment", helperFile)
	}
}

var helperSentence = regexp.MustCompile(`IsolateHostCredentials`)

func TestC2138_006_TheSpecAndThePackageNoteStateTheChangeInSte(t *testing.T) {
	root := acsassert.RepoRoot(t)
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	for _, doc := range []string{
		"docs/architecture/packages/internal-gittest.md",
		"docs/architecture/test-system.md",
	} {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(doc)))
		if err != nil {
			t.Fatalf("read %s: %v", doc, err)
		}
		var stated []string
		for _, line := range strings.Split(string(raw), "\n") {
			if helperSentence.MatchString(line) {
				stated = append(stated, line)
			}
		}
		if len(stated) == 0 {
			t.Errorf("%s does not state IsolateHostCredentials", doc)
			continue
		}
		joined := strings.Join(stated, "\n")
		if !strings.Contains(doc, "test-system") && !strings.Contains(joined, "GH_CONFIG_DIR") {
			t.Errorf("%s names the helper but not GH_CONFIG_DIR, the value that hides the gh login", doc)
		}
		for _, f := range stelint.Check([]byte(joined), stelint.Options{Words: words}) {
			if !f.IsSkipNotice() {
				t.Errorf("%s: STE finding %s: %s: %s", doc, f.Rule, f.Message, f.Excerpt)
			}
		}
	}
}

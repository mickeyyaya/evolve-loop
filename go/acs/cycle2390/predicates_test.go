//go:build acs

package cycle2390

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const validPredicate = "package cycle9001\n\nimport \"testing\"\n\nfunc TestC9001_001_Only(t *testing.T) {}\n"

const hiddenName = "TestHidden_SECRET_Predicate"

const hiddenPredicate = "package hidden\n\nimport \"testing\"\n\nfunc TestHidden_SECRET_Predicate(t *testing.T) {}\n"

type site struct {
	worktree string
	pkg      string
}

func newSite(t *testing.T) site {
	t.Helper()
	worktree := t.TempDir()
	pkg := filepath.Join(worktree, "go", "acs", "cycle9001")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "a_test.go"), []byte(validPredicate), 0o644); err != nil {
		t.Fatal(err)
	}
	return site{worktree: worktree, pkg: pkg}
}

type outcome struct {
	req handoff.Request
	err error
}

func buildRequest(t *testing.T, s site) outcome {
	t.Helper()
	done := make(chan outcome, 1)
	go func() {
		req, err := request.ForPair(request.PairTDDBuild, request.Site{Worktree: s.worktree, Cycle: 9001})
		done <- outcome{req, err}
	}()
	select {
	case got := <-done:
		return got
	case <-time.After(20 * time.Second):
		t.Fatal("the tdd-build request did not return: it reads a file that is not a plain file")
		return outcome{}
	}
}

func listsName(got outcome, name string) bool {
	for _, ask := range got.req.Asks {
		if ask.ID == name {
			return true
		}
	}
	return false
}

func TestC2390_001_AFifoTestFileDoesNotHangTheTDDBuildRequest(t *testing.T) {
	s := newSite(t)
	fifo := filepath.Join(s.pkg, "pipe_test.go")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skipf("no fifo support: %v", err)
	}
	t.Cleanup(func() {
		if f, err := os.OpenFile(fifo, os.O_RDWR, 0); err == nil {
			_ = f.Close()
		}
	})
	got := buildRequest(t, s)
	if got.err == nil && !listsName(got, "TestC9001_001_Only") {
		t.Errorf("the regular predicate beside the fifo is not listed: %+v", got.req.Asks)
	}
}

func TestC2390_002_ASymlinkedTestFileAddsNothingToTheTDDBuildRequest(t *testing.T) {
	s := newSite(t)
	hidden := filepath.Join(t.TempDir(), "hidden.go")
	if err := os.WriteFile(hidden, []byte(hiddenPredicate), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(hidden, filepath.Join(s.pkg, "link_test.go")); err != nil {
		t.Fatal(err)
	}
	got := buildRequest(t, s)
	if listsName(got, hiddenName) {
		t.Errorf("a symlinked test file contributed %s to the request", hiddenName)
	}
	if got.err == nil && !listsName(got, "TestC9001_001_Only") {
		t.Errorf("the regular predicate beside the symlink is not listed: %+v", got.req.Asks)
	}
}

func TestC2390_011_SeedTaskContractReturnsWithAFifoAndASymlinkBesideAPredicate(t *testing.T) {
	moduleDir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatal(err)
	}
	run := exec.Command("go", "test", "-count=1", "-v", "-run", "^TestPlainFiles_", "./internal/core")
	run.Dir = moduleDir
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("the seedTaskContract plain-file tests are red: %v\n%s", err, acsassert.OutputTail(string(out), 40))
	}
	for _, name := range []string{"FifoTestFileDoesNotHangTheBuildPrompt", "SymlinkedTestFileAddsNoBytesToTheBuildPrompt"} {
		if !strings.Contains(string(out), "--- PASS: TestPlainFiles_"+name) {
			t.Errorf("TestPlainFiles_%s did not run and pass", name)
		}
	}
}

func TestC2390_003_ALedgerLargerThanTheCapIsRefused(t *testing.T) {
	ledger := filepath.Join(t.TempDir(), "handoff-tdd-build.json")
	f, err := os.Create(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(256 << 20); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := core.ReadHandoffLedger(ledger)
	if err == nil {
		t.Fatalf("a 256 MiB ledger was read whole (%d bytes)", len(raw))
	}
}

func TestC2390_004_AnOrdinaryLedgerIsStillRead(t *testing.T) {
	ledger := filepath.Join(t.TempDir(), "handoff-tdd-build.json")
	body := `{"schema_version":1,"pair":"tdd-build"}`
	if err := os.WriteFile(ledger, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := core.ReadHandoffLedger(ledger)
	if err != nil || string(raw) != body {
		t.Fatalf("ReadHandoffLedger = (%q, %v), want the ledger bytes", raw, err)
	}
}

func coverOf(t *testing.T, moduleDir, pkg, run string) string {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "cover.out")
	test := exec.Command("go", "test", "-count=1", "-run", run, "-coverprofile", profile, pkg)
	test.Dir = moduleDir
	if out, err := test.CombinedOutput(); err != nil {
		t.Fatalf("go test %s -run %s: %v\n%s", pkg, run, err, acsassert.OutputTail(string(out), 30))
	}
	fn := exec.Command("go", "tool", "cover", "-func", profile)
	fn.Dir = moduleDir
	out, err := fn.CombinedOutput()
	if err != nil {
		t.Fatalf("go tool cover: %v\n%s", err, out)
	}
	return string(out)
}

func TestC2390_005_EachChangedFunctionIsFullyCovered(t *testing.T) {
	moduleDir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		pkg, run string
		funcs    []string
		optional []string
	}{
		{"./internal/handoff/request", "TestTDDBuild", []string{"TDDBuild"}, nil},
		{"./internal/core", "TestReadHandoffLedger|TestPlainFiles", []string{"ReadHandoffLedger"}, nil},
		{"./internal/plainfile", ".", []string{"Open", "Read", "ReadWith"}, nil},
	}
	for _, c := range cases {
		report := coverOf(t, moduleDir, c.pkg, c.run)
		for _, fn := range c.funcs {
			pct, err := acsassert.FuncCoverage(report, fn)
			if err != nil {
				t.Errorf("%s: %v", fn, err)
				continue
			}
			if pct < 100 {
				t.Errorf("%s coverage = %.1f%%, want 100%%", fn, pct)
			}
		}
		for _, fn := range c.optional {
			if pct, err := acsassert.FuncCoverage(report, fn); err == nil && pct < 100 {
				t.Errorf("%s coverage = %.1f%%, want 100%%", fn, pct)
			}
		}
	}
}

func commentsOutsideTheDirectives(t *testing.T, path string) []string {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	var found []string
	for _, group := range parsed.Comments {
		if group == parsed.Doc {
			continue
		}
		for _, c := range group.List {
			if !strings.HasPrefix(c.Text, "//go:") {
				found = append(found, c.Text)
			}
		}
	}
	return found
}

func TestC2390_006_TheChangedFilesCarryNoComments(t *testing.T) {
	root := acsassert.RepoRoot(t)
	files := []string{"go/internal/handoff/request/request.go", "go/internal/core/handoff_reviewer.go", "go/internal/core/handoff_persona.go"}
	plain, err := filepath.Glob(filepath.Join(root, "go", "internal", "plainfile", "*.go"))
	if err != nil || len(plain) == 0 {
		t.Fatalf("the plainfile package has no source files: %v", err)
	}
	for _, rel := range files {
		plain = append(plain, filepath.Join(root, rel))
	}
	for _, path := range plain {
		for _, text := range commentsOutsideTheDirectives(t, path) {
			t.Errorf("%s carries a comment: %s", path, text)
		}
	}
}

func goDeps(t *testing.T, pkg string) string {
	t.Helper()
	moduleDir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatal(err)
	}
	list := exec.Command("go", "list", "-deps", pkg)
	list.Dir = moduleDir
	out, err := list.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", pkg, err, acsassert.OutputTail(string(out), 30))
	}
	return string(out)
}

const plainfilePackage = "github.com/mickeyyaya/evolve-loop/go/internal/plainfile"

func TestC2390_007_TheTDDBuildRequestReadsEachTestFileThroughThePlainFileReader(t *testing.T) {
	if !strings.Contains(goDeps(t, "./internal/handoff/request"), plainfilePackage) {
		t.Errorf("the handoff request package does not depend on %s", plainfilePackage)
	}
	path := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "handoff", "request", "request.go")
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	ast.Inspect(parsed, func(n ast.Node) bool {
		call, isCall := n.(*ast.CallExpr)
		if !isCall {
			return true
		}
		selector, isSelector := call.Fun.(*ast.SelectorExpr)
		if !isSelector {
			return true
		}
		switch selector.Sel.Name {
		case "ParseFile":
			calls++
			if len(call.Args) < 3 || isNilIdent(call.Args[2]) {
				t.Errorf("request.go:%d parses a file by path: the parser reopens the file after the check", fset.Position(call.Pos()).Line)
			}
		case "ReadFile":
			t.Errorf("request.go reads a file by path with %s", selector.Sel.Name)
		}
		return true
	})
	if calls == 0 {
		t.Error("request.go no longer parses the test files, so the check proves nothing")
	}
}

func isNilIdent(expr ast.Expr) bool {
	ident, isIdent := expr.(*ast.Ident)
	return isIdent && ident.Name == "nil"
}

func TestC2390_008_TheHandoffLedgerReadGoesThroughThePlainFileReader(t *testing.T) {
	if !strings.Contains(goDeps(t, "./internal/core"), plainfilePackage) {
		t.Errorf("core does not depend on %s", plainfilePackage)
	}
	path := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "core", "handoff_reviewer.go")
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	reached := false
	ast.Inspect(parsed, func(n ast.Node) bool {
		selector, isSelector := n.(*ast.SelectorExpr)
		if !isSelector {
			return true
		}
		if pkg, isIdent := selector.X.(*ast.Ident); isIdent && pkg.Name == "plainfile" {
			reached = true
		}
		if selector.Sel.Name == "SameFile" || selector.Sel.Name == "O_NONBLOCK" {
			t.Errorf("handoff_reviewer.go repeats the open and identity logic of the plain-file reader: %s", selector.Sel.Name)
		}
		return true
	})
	if !reached {
		t.Error("handoff_reviewer.go does not call the plain-file reader")
	}
}

func TestC2390_009_TheHardLinkTestHasOneHome(t *testing.T) {
	root := acsassert.RepoRoot(t)
	plainSource := filepath.Join(root, "go", "internal", "plainfile", "plainfile.go")
	body, err := os.ReadFile(plainSource)
	if err != nil {
		t.Fatalf("the plain-file reader has no source file: %v", err)
	}
	if !strings.Contains(string(body), "HardLinked(") {
		t.Error("the plain-file reader does not reuse inputindex.HardLinked")
	}
	for _, rel := range []string{"go/internal/plainfile/plainfile.go", "go/internal/handoff/request/request.go", "go/internal/core/handoff_reviewer.go"} {
		source, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(source), "Nlink") {
			t.Errorf("%s re-implements the hard-link test of inputindex.HardLinked", rel)
		}
	}
}

func TestC2390_010_ThePlainFilePackageIsEnrolledInTheApiCoverGate(t *testing.T) {
	enrolled, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), "go", ".apicover-enforce"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(enrolled), "\n") {
		if strings.TrimSpace(line) == "./internal/plainfile" {
			return
		}
	}
	t.Error("go/.apicover-enforce does not list ./internal/plainfile")
}

//go:build acs

package cycle2672

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/inboxcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/landcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/derived"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxtest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	guardTestName          = "TestNoCLIPackageStoresDepsInAProcessGlobal"
	inboxParallelFloor     = 130
	offenderSource         = "package probe\n\nimport \"sync/atomic\"\n\ntype Deps struct{}\n\nvar hostHolder atomic.Pointer[Deps]\n"
	benignSource           = "package probe\n\nimport \"sync/atomic\"\n\ntype Config struct{}\n\nvar hits atomic.Int64\n\nvar cached atomic.Pointer[Config]\n"
	valueTypedOffender     = "package probe\n\nimport \"sync/atomic\"\n\ntype hostDeps struct{}\n\nvar (\n\tunrelated int\n\tcurrent   = atomic.Pointer[hostDeps]{}\n)\n"
	probeDirPrefix         = "zzacs2672probe"
	pausePatternTopLevel   = `(?m)^=== PAUSE (Test[A-Za-z0-9_]*)$`
	docsTestsName          = "TestRunCI_DocsTests"
	goTestFailMarker       = "--- FAIL"
	noTestsMarker          = "no tests to run"
	cliRel                 = "internal/cli"
	baselineInboxParallels = 107
)

func cliDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.GoModuleDir(t), filepath.FromSlash(cliRel))
}

func isAtomicPointerOfDeps(expr ast.Expr) bool {
	index, ok := expr.(*ast.IndexExpr)
	if !ok {
		return false
	}
	sel, ok := index.X.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Pointer" {
		return false
	}
	if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "atomic" {
		return false
	}
	switch arg := index.Index.(type) {
	case *ast.Ident:
		return strings.HasSuffix(strings.ToLower(arg.Name), "deps")
	case *ast.SelectorExpr:
		return strings.HasSuffix(strings.ToLower(arg.Sel.Name), "deps")
	}
	return false
}

func atomicDepsGlobals(t *testing.T, filename string, src any) []string {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), filename, src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}
	var found []string
	for _, decl := range parsed.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			value := spec.(*ast.ValueSpec)
			hit := value.Type != nil && isAtomicPointerOfDeps(value.Type)
			for _, v := range value.Values {
				if lit, ok := v.(*ast.CompositeLit); ok && isAtomicPointerOfDeps(lit.Type) {
					hit = true
				}
			}
			if hit {
				for _, name := range value.Names {
					found = append(found, filename+":"+name.Name)
				}
			}
		}
	}
	return found
}

func walkGoFiles(t *testing.T, root string, wantTests bool) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") != wantTests {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func topLevelTests(t *testing.T, dir string) map[string]bool {
	t.Helper()
	tests := map[string]bool{}
	for _, file := range walkGoFiles(t, dir, true) {
		if filepath.Dir(file) != dir {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Test") || fn.Name.Name == "TestMain" {
				continue
			}
			tests[fn.Name.Name] = callsParallelAtTopLevel(fn.Body)
		}
	}
	return tests
}

func callsParallelAtTopLevel(body *ast.BlockStmt) bool {
	for _, stmt := range body.List {
		expr, ok := stmt.(*ast.ExprStmt)
		if !ok {
			continue
		}
		call, ok := expr.X.(*ast.CallExpr)
		if !ok {
			continue
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Parallel" {
			return true
		}
	}
	return false
}

func parallelCount(tests map[string]bool) int {
	n := 0
	for _, parallel := range tests {
		if parallel {
			n++
		}
	}
	return n
}

func pausedTopLevelTests(output string) map[string]bool {
	paused := map[string]bool{}
	for _, m := range regexp.MustCompile(pausePatternTopLevel).FindAllStringSubmatch(output, -1) {
		paused[m[1]] = true
	}
	return paused
}

func raceRun(t *testing.T, pkgRel string, extra ...string) (string, bool) {
	t.Helper()
	args := append([]string{"test", "-race", "-count=1"}, extra...)
	args = append(args, "./"+pkgRel)
	stdout, stderr, code, err := acsassert.SubprocessOutputIn(acsassert.GoModuleDir(t), "go", args...)
	out := stdout + stderr
	if err != nil || code != 0 || strings.Contains(out, goTestFailMarker) {
		t.Errorf("go %s: exit=%d err=%v\n%s", strings.Join(args, " "), code, err, acsassert.OutputTail(out, 40))
		return out, false
	}
	return out, true
}

func TestC2672_001_NoCLIPackageDeclaresAnAtomicDepsPointer(t *testing.T) {
	var offenders []string
	for _, file := range walkGoFiles(t, cliDir(t), false) {
		offenders = append(offenders, atomicDepsGlobals(t, file, nil)...)
	}
	if len(offenders) != 0 {
		t.Errorf("a package-level atomic Deps pointer remains under go/internal/cli: %v", offenders)
	}
}

func TestC2672_002_TheScannerSeesEachAtomicDepsShapeAndNoBenignOne(t *testing.T) {
	if got := atomicDepsGlobals(t, "offender.go", offenderSource); len(got) != 1 {
		t.Errorf("a renamed atomic.Pointer[Deps] global: got %v, want exactly one", got)
	}
	if got := atomicDepsGlobals(t, "typed.go", valueTypedOffender); len(got) != 1 {
		t.Errorf("an atomic.Pointer literal of a *Deps type: got %v, want exactly one", got)
	}
	if got := atomicDepsGlobals(t, "benign.go", benignSource); len(got) != 0 {
		t.Errorf("an atomic counter and an atomic.Pointer[Config]: got %v, want none", got)
	}
}

func guardPackageRel(t *testing.T) (string, bool) {
	t.Helper()
	cli := cliDir(t)
	for _, file := range walkGoFiles(t, cli, true) {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range parsed.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == guardTestName {
				rel, err := filepath.Rel(acsassert.GoModuleDir(t), filepath.Dir(file))
				if err != nil {
					t.Fatal(err)
				}
				return filepath.ToSlash(rel), true
			}
		}
	}
	return "", false
}

func runGuard(t *testing.T, pkgRel string) (string, int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutputIn(acsassert.GoModuleDir(t), "go", "test", "-count=1", "-v", "-run", "^"+guardTestName+"$", "./"+pkgRel)
	return stdout + stderr, code
}

func withProbe(t *testing.T, name, source string) string {
	t.Helper()
	dir := filepath.Join(cliDir(t), probeDirPrefix+name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	if err := os.WriteFile(filepath.Join(dir, "probe.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestC2672_003_TheGuardTestExistsPassesOnTheTreeAndFailsOnAnOffender(t *testing.T) {
	pkgRel, found := guardPackageRel(t)
	if !found {
		t.Fatalf("no %s in any _test.go file under go/internal/cli", guardTestName)
	}
	out, code := runGuard(t, pkgRel)
	if code != 0 || strings.Contains(out, noTestsMarker) || !strings.Contains(out, "--- PASS: "+guardTestName) {
		t.Fatalf("the guard does not pass on the tree (exit %d):\n%s", code, acsassert.OutputTail(out, 30))
	}
	t.Run("offender", func(t *testing.T) {
		probe := withProbe(t, "offender", offenderSource)
		out, code := runGuard(t, pkgRel)
		if code == 0 || !strings.Contains(out, filepath.Base(probe)) {
			t.Errorf("the guard did not fail on, and name, an atomic.Pointer[Deps] global in %s (exit %d):\n%s", filepath.Base(probe), code, acsassert.OutputTail(out, 30))
		}
	})
	t.Run("benign", func(t *testing.T) {
		withProbe(t, "benign", benignSource)
		out, code := runGuard(t, pkgRel)
		if code != 0 {
			t.Errorf("the guard fired on an atomic counter and an atomic.Pointer[Config] (exit %d):\n%s", code, acsassert.OutputTail(out, 30))
		}
	})
}

func TestC2672_004_ALandcmdRunKeepsItsDepsWhenAnotherRunStartsAndEndsMidFlight(t *testing.T) {
	root := t.TempDir()
	entered := make(chan struct{})
	release := make(chan struct{})
	var firstRunRefreshes atomic.Int32
	firstRun := landcmd.Deps{
		ResolveRoot: func(string, io.Writer) (string, error) {
			close(entered)
			<-release
			return root, nil
		},
		RefreshWorktree: func(string) derived.Worktree {
			firstRunRefreshes.Add(1)
			return derived.Worktree{}
		},
	}
	done := make(chan int)
	go func() {
		done <- landcmd.RunPR([]string{"refresh", "--project-root", root}, nil, io.Discard, io.Discard, firstRun)
	}()
	<-entered
	landcmd.RunPR(nil, nil, io.Discard, io.Discard, landcmd.Deps{})
	close(release)
	<-done
	if got := firstRunRefreshes.Load(); got != 1 {
		t.Errorf("the first run's RefreshWorktree ran %d times, want 1: the second run replaced its deps", got)
	}
}

func TestC2672_005_AnInboxcmdRunKeepsItsDepsWhenAnEarlierRunReturnsFirst(t *testing.T) {
	root := inboxtest.EmptyRoot(t)
	aEntered, aRelease := make(chan struct{}), make(chan struct{})
	bEntered, bRelease := make(chan struct{}), make(chan struct{})
	var secondRunLaneChecks atomic.Int32
	depsA := inboxcmd.Deps{
		CheckRootHasEvolve: func(string, string, io.Writer) int {
			close(aEntered)
			<-aRelease
			return 0
		},
		LaneForbidden: func(string, io.Writer) func(string) bool { return func(string) bool { return false } },
	}
	depsB := inboxcmd.Deps{
		CheckRootHasEvolve: func(string, string, io.Writer) int {
			close(bEntered)
			<-bRelease
			return 0
		},
		LaneForbidden: func(string, io.Writer) func(string) bool {
			secondRunLaneChecks.Add(1)
			return func(string) bool { return false }
		},
	}
	args := []string{"list", "--project-root", root}
	aDone := make(chan int)
	bDone := make(chan any)
	go func() { aDone <- inboxcmd.Run(args, nil, io.Discard, io.Discard, depsA) }()
	<-aEntered
	go func() {
		defer func() { bDone <- recover() }()
		inboxcmd.Run(args, nil, io.Discard, io.Discard, depsB)
	}()
	<-bEntered
	close(aRelease)
	<-aDone
	close(bRelease)
	if panicked := <-bDone; panicked != nil {
		t.Errorf("the second run panicked after the first run returned: %v", panicked)
	}
	if got := secondRunLaneChecks.Load(); got != 1 {
		t.Errorf("the second run's LaneForbidden ran %d times, want 1: the first run's return replaced its deps", got)
	}
}

func TestC2672_006_TestRunCIDocsTestsRunsInParallel(t *testing.T) {
	tests := topLevelTests(t, filepath.Join(cliDir(t), "landcmd"))
	parallel, present := tests[docsTestsName]
	if !present {
		t.Fatalf("%s is gone from landcmd", docsTestsName)
	}
	if !parallel {
		t.Errorf("%s does not call t.Parallel", docsTestsName)
	}
}

func TestC2672_007_TheInboxcmdTestsRunInParallel(t *testing.T) {
	got := parallelCount(topLevelTests(t, filepath.Join(cliDir(t), "inboxcmd")))
	if got < inboxParallelFloor {
		t.Errorf("%d inboxcmd tests call t.Parallel, want at least %d (%d did before)", got, inboxParallelFloor, baselineInboxParallels)
	}
}

func TestC2672_008_LandcmdPassesUnderRaceAndPausesItsDocsTestsForParallelRuns(t *testing.T) {
	out, ok := raceRun(t, "internal/cli/landcmd", "-v")
	if !ok {
		return
	}
	if !pausedTopLevelTests(out)[docsTestsName] {
		t.Errorf("%s never paused at run time, so it did not run in parallel", docsTestsName)
	}
}

func TestC2672_009_InboxcmdPassesUnderRaceAndRunsItsTestsInParallel(t *testing.T) {
	out, ok := raceRun(t, "internal/cli/inboxcmd", "-v")
	if !ok {
		return
	}
	if got := len(pausedTopLevelTests(out)); got < inboxParallelFloor {
		t.Errorf("%d inboxcmd tests paused at run time, want at least %d", got, inboxParallelFloor)
	}
}

func TestC2672_010_EveryOtherChangedCLIPackagePassesUnderRace(t *testing.T) {
	module := acsassert.GoModuleDir(t)
	var changed []string
	for _, base := range []string{"refs/remotes/origin/main", "refs/heads/main"} {
		mergeBase, _, code, err := acsassert.SubprocessOutputIn(module, "git", "merge-base", "HEAD", base)
		if err != nil || code != 0 {
			continue
		}
		diff, _, _, _ := acsassert.SubprocessOutputIn(module, "git", "diff", "--name-only", strings.TrimSpace(mergeBase), "--", cliRel)
		changed = strings.Fields(diff)
		break
	}
	seen := map[string]bool{"internal/cli/landcmd": true, "internal/cli/inboxcmd": true}
	for _, file := range changed {
		file = strings.TrimPrefix(file, "go/")
		dir := filepath.ToSlash(filepath.Dir(file))
		if seen[dir] || !strings.HasPrefix(dir, cliRel+"/") || strings.Contains(dir, probeDirPrefix) {
			continue
		}
		seen[dir] = true
		if _, err := os.Stat(filepath.Join(module, filepath.FromSlash(dir))); err != nil {
			continue
		}
		t.Run(dir, func(t *testing.T) { raceRun(t, dir) })
	}
}

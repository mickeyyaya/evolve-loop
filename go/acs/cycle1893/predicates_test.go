//go:build acs

package cycle1893

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type gitRun struct {
	stdout, stderr string
	code           int
}

func runGit(t *testing.T, dir string, args ...string) gitRun {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return gitRun{out.String(), errb.String(), 0}
	case errors.As(err, &exitErr):
		return gitRun{out.String(), errb.String(), exitErr.ExitCode()}
	default:
		t.Fatalf("git %v did not run: %v", args, err)
		return gitRun{}
	}
}

func committedRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.Fixture(t)
	r.Git("commit", "-q", "--allow-empty", "-m", "base")
	return r
}

func TestC1893_001_FaultOnFailsTheTargetedVerb(t *testing.T) {
	r := committedRepo(t)

	gittest.FaultOn(t, "status")
	got := runGit(t, r.Dir, "status", "--porcelain")

	if got.code != 128 {
		t.Errorf("faulted git status exit = %d, want 128", got.code)
	}
	if !strings.Contains(got.stderr, "fatal:") || !strings.Contains(got.stderr, "status") {
		t.Errorf("faulted git status stderr = %q, want a fatal: line naming the verb", got.stderr)
	}
}

func TestC1893_002_FaultOnForwardsEveryOtherVerbToTheRealGit(t *testing.T) {
	r := committedRepo(t)
	headBeforeFault := r.Git("rev-parse", "HEAD")

	gittest.FaultOn(t, "show")
	head := runGit(t, r.Dir, "rev-parse", "HEAD")
	showRefIsNotShow := runGit(t, r.Dir, "show-ref", "--verify", "refs/heads/main")
	shown := runGit(t, r.Dir, "show", "-s", "--format=%s", "HEAD")

	if head.code != 0 || strings.TrimSpace(head.stdout) != headBeforeFault {
		t.Errorf("forwarded rev-parse = %+v, want exit 0 and %s", head, headBeforeFault)
	}
	if showRefIsNotShow.code != 0 || !strings.Contains(showRefIsNotShow.stdout, headBeforeFault) {
		t.Errorf("show-ref under a show fault = %+v, want the real git's answer: the verb matches a whole argument only", showRefIsNotShow)
	}
	if shown.code == 0 {
		t.Errorf("git show under a show fault = %+v, want the injected failure", shown)
	}
}

func TestC1893_003_InstallFaultSkipsTheFirstNCallsThenFailsEveryLaterOne(t *testing.T) {
	cases := []struct {
		name      string
		skip      int
		wantCodes []int
	}{
		{"skip zero fails the first call", 0, []int{3, 3}},
		{"skip two lets two calls through", 2, []int{0, 0, 3, 3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := committedRepo(t)

			gittest.InstallFault(t, gittest.GitFault{Verb: "status", Skip: tc.skip, Code: 3})

			for i, want := range tc.wantCodes {
				if got := runGit(t, r.Dir, "status", "--porcelain"); got.code != want {
					t.Errorf("status call %d exit = %d (stderr %q), want %d", i+1, got.code, got.stderr, want)
				}
			}
		})
	}
}

func TestC1893_004_InstallFaultWithCodeZeroFakesTheVerbsOutput(t *testing.T) {
	r := committedRepo(t)
	if real := runGit(t, r.Dir, "status", "--porcelain"); real.code != 0 || real.stdout != "" {
		t.Fatalf("precondition: clean fixture status = %+v", real)
	}

	gittest.InstallFault(t, gittest.GitFault{Verb: "status", Stdout: "?? blk/x"})
	got := runGit(t, r.Dir, "status", "--porcelain")

	if got.code != 0 || got.stdout != "?? blk/x" {
		t.Errorf("faked status = %+v, want exit 0 and stdout %q", got, "?? blk/x")
	}
}

func TestC1893_010_InstallFaultWritesTheDeclaredStderr(t *testing.T) {
	r := committedRepo(t)

	gittest.InstallFault(t, gittest.GitFault{Verb: "show-ref", Code: 128, Stderr: "fatal: bad object"})
	got := runGit(t, r.Dir, "show-ref", "--verify", "refs/heads/main")

	if got.code != 128 || strings.TrimSpace(got.stderr) != "fatal: bad object" {
		t.Errorf("faulted show-ref = %+v, want exit 128 and stderr %q", got, "fatal: bad object")
	}
	if got.stdout != "" {
		t.Errorf("faulted show-ref stdout = %q, want none: the real git must not run", got.stdout)
	}
}

func TestC1893_005_BranchExistsAnswersForBranchesOnly(t *testing.T) {
	r := committedRepo(t)
	r.Git("branch", "feature")
	r.Git("tag", "v1")
	notARepo := t.TempDir()

	cases := []struct {
		name, dir, branch string
		want              bool
	}{
		{"current branch", r.Dir, "main", true},
		{"other branch", r.Dir, "feature", true},
		{"absent branch", r.Dir, "nope", false},
		{"a tag is not a branch", r.Dir, "v1", false},
		{"empty name", r.Dir, "", false},
		{"directory outside any repo", notARepo, "main", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := gittest.BranchExists(t, tc.dir, tc.branch); got != tc.want {
				t.Errorf("BranchExists(%q) = %v, want %v", tc.branch, got, tc.want)
			}
		})
	}
}

func TestC1893_006_BranchExistsSeesADeletedBranchGone(t *testing.T) {
	r := committedRepo(t)
	r.Git("branch", "doomed")
	if !gittest.BranchExists(t, r.Dir, "doomed") {
		t.Fatal("BranchExists missed a branch that was just created")
	}

	r.Git("branch", "-D", "doomed")

	if gittest.BranchExists(t, r.Dir, "doomed") {
		t.Error("BranchExists still reports a deleted branch")
	}
}

func TestC1893_007_TheKitTestsTheEvalNamesPass(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmd := exec.Command("go", "test", "-count=1", "-v", "-run",
		"^(TestFaultOn_VerbFails|TestFaultOn_NonTargetedVerbPasses|TestFaultOn_SkipCount|TestBranchExists)$",
		"./internal/gittest")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test ./internal/gittest: %v\n%s", err, out)
	}
	for _, name := range []string{"TestFaultOn_VerbFails", "TestFaultOn_NonTargetedVerbPasses", "TestFaultOn_SkipCount", "TestBranchExists"} {
		if !strings.Contains(string(out), "--- PASS: "+name+" ") {
			t.Errorf("%s did not pass in internal/gittest:\n%s", name, out)
		}
	}
}

var e3MigratedFiles = []string{
	"cmd_worktree_dev_cleanup_backfill_e3_test.go",
	"cmd_worktree_dev_cleanup_backfill_e3_integration_test.go",
	"cmd_worktree_dev_backfill_e3_integration_test.go",
	"cmd_worktree_stale_backfill_e3_test.go",
}

type e3Usage struct {
	rawGit   []string
	kitCalls map[string]bool
}

func scanE3File(t *testing.T, path string, use *e3Usage) {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	base := filepath.Base(path)
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Name.Name == "e3FaultyGit" {
				use.rawGit = append(use.rawGit, base+": declares its own e3FaultyGit shim")
			}
		case *ast.Ident:
			if x.Name == "realGit" || x.Name == "brBranchExists" || x.Name == "bfStub" {
				use.rawGit = append(use.rawGit, base+": uses "+x.Name)
			}
		case *ast.CallExpr:
			sel, ok := x.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if pkg.Name == "gittest" {
				use.kitCalls[sel.Sel.Name] = true
			}
			if pkg.Name == "exec" && (sel.Sel.Name == "Command" || sel.Sel.Name == "LookPath") && len(x.Args) > 0 {
				if lit, ok := x.Args[0].(*ast.BasicLit); ok {
					if s, err := strconv.Unquote(lit.Value); err == nil && s == "git" {
						use.rawGit = append(use.rawGit, base+": exec."+sel.Sel.Name+"(\"git\")")
					}
				}
			}
		}
		return true
	})
}

// acs-predicate: source-structure
func TestC1893_008_E3WorktreeTestsUseTheKitAndNoRawGit(t *testing.T) {
	root := acsassert.RepoRoot(t)
	use := &e3Usage{kitCalls: map[string]bool{}}
	for _, name := range e3MigratedFiles {
		scanE3File(t, filepath.Join(root, "go", "cmd", "evolve", name), use)
	}

	if len(use.rawGit) > 0 {
		t.Errorf("E3 worktree tests still drive the real git by hand:\n%s", strings.Join(use.rawGit, "\n"))
	}
	if !use.kitCalls["FaultOn"] && !use.kitCalls["InstallFault"] {
		t.Errorf("no E3 worktree test injects its git fault through gittest.FaultOn or gittest.InstallFault (kit calls: %v)", use.kitCalls)
	}
	if !use.kitCalls["BranchExists"] {
		t.Errorf("no E3 worktree test checks a branch through gittest.BranchExists (kit calls: %v)", use.kitCalls)
	}
}

func TestC1893_009_MigratedE3WorktreeTestsStillPass(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmd := exec.Command("go", "test", "-count=1", "-tags", "integration", "-run",
		"^(TestWorktreeDevCleanup|TestWorktreeCleanupStale_|TestRemoveStaleLane_|TestRunWorktreeCreateDev_)", "./cmd/evolve")
	cmd.Dir = filepath.Join(root, "go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("migrated E3 worktree tests: %v\n%s", err, out)
	}
}

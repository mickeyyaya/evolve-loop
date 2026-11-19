//go:build acs

package cycle2543

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	hostConfigPlanted = "/host/gitconfig"
	hostTokenPlanted  = "host-token-planted-by-predicate"
	originMain        = "refs/remotes/origin/main"
)

var handWrittenSites = []string{
	"go/internal/changedpkgs/fromgit_test.go",
	"go/internal/changedpkgs/changedpkgs_derivability_test.go",
	"go/internal/dossier/rollback_test.go",
	"go/internal/core/build_removal_check_index_test.go",
	"go/internal/core/cycle_source_integration_test.go",
	"go/internal/core/orchestrator_signal_checkpoint_integration_test.go",
	"go/cmd/evolve/cmd_loop_gc_worktree_test.go",
}

var packageNotes = []string{
	"docs/architecture/packages/internal-changedpkgs.md",
	"docs/architecture/packages/internal-dossier.md",
	"docs/architecture/packages/internal-core.md",
	"docs/architecture/packages/cmd-evolve.md",
}

func goTest(t *testing.T, pkg, runPattern string) (string, int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-run", runPattern, pkg)
	if code < 0 {
		t.Fatalf("go test %s did not run: %v", pkg, err)
	}
	return stdout + stderr, code
}

func requireTestRan(t *testing.T, out string, code int, pkg string) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Fatalf("RED: no test matched in %s\n%s", pkg, acsassert.OutputTail(out, 20))
	}
	if code != 0 {
		t.Errorf("RED: go test %s failed (exit=%d)\n%s", pkg, code, acsassert.OutputTail(out, 30))
	}
}

func TestC2543_001_TheNamedRedTestOfTheHelperStaysGreen(t *testing.T) {
	out, code := goTest(t, "./internal/gittest", "^TestIsolateHostCredentials_ARealGhInATestSeesNoLogin$")
	requireTestRan(t, out, code, "./internal/gittest")
}

func adoptedTestMain(t *testing.T, pkg, testName string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", hostConfigPlanted)
	t.Setenv("GH_TOKEN", hostTokenPlanted)
	t.Setenv("GITHUB_TOKEN", hostTokenPlanted)
	out, code := goTest(t, pkg, "^"+testName+"$")
	requireTestRan(t, out, code, pkg)
}

func TestC2543_002_ChangedpkgsTestMainIsolatesHostCredentials(t *testing.T) {
	adoptedTestMain(t, "./internal/changedpkgs", "TestMain_IsolatesHostCredentialsForEveryTestInChangedpkgs")
}

func TestC2543_003_DossierTestMainIsolatesHostCredentials(t *testing.T) {
	adoptedTestMain(t, "./internal/dossier", "TestMain_IsolatesHostCredentialsForEveryTestInDossier")
}

func TestC2543_004_CoreTestMainIsolatesHostCredentials(t *testing.T) {
	adoptedTestMain(t, "./internal/core", "TestMain_IsolatesHostCredentialsForEveryTestInCore")
}

func TestC2543_005_NoHandWrittenGitConfigLineRemainsInTheAdoptingTests(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range handWrittenSites {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("preservation: %s must stay: %v", rel, err)
		}
		for i, line := range strings.Split(string(body), "\n") {
			if strings.Contains(line, "GIT_CONFIG_GLOBAL") || strings.Contains(line, "GIT_CONFIG_SYSTEM") {
				t.Errorf("RED: %s:%d keeps a hand-written git config line: %s", rel, i+1, strings.TrimSpace(line))
			}
		}
	}
}

func TestC2543_006_TheHelperKeepsFullStatementCoverage(t *testing.T) {
	root := acsassert.RepoRoot(t)
	profile := filepath.Join(t.TempDir(), "cover.out")
	goDir := filepath.Join(root, "go")
	_, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-coverprofile="+profile, "./internal/gittest")
	if code != 0 {
		t.Fatalf("go test ./internal/gittest failed: %v\n%s", err, acsassert.OutputTail(stderr, 30))
	}
	funcs, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "tool", "cover", "-func="+profile)
	if code != 0 {
		t.Fatalf("go tool cover failed: %v\n%s", err, acsassert.OutputTail(stderr, 30))
	}
	pct, err := acsassert.FuncCoverage(funcs, "IsolateHostCredentials")
	if err != nil || pct < 100 {
		t.Errorf("RED: IsolateHostCredentials coverage = %.1f%% (err=%v), want 100%%", pct, err)
	}
}

func addedLines(t *testing.T) map[string][]string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	base, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", originMain)
	if err != nil || code != 0 {
		t.Fatalf("merge-base failed (exit=%d err=%v)", code, err)
	}
	diff, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "-U0", strings.TrimSpace(base), "--", "go/internal", "go/cmd")
	if err != nil || code != 0 {
		t.Fatalf("git diff failed (exit=%d err=%v)", code, err)
	}
	added := map[string][]string{}
	var current string
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++ b/"):
			current = strings.TrimPrefix(line, "+++ b/")
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			added[current] = append(added[current], line[1:])
		}
	}
	untracked, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--others", "--exclude-standard", "--", "go/internal", "go/cmd")
	if err != nil || code != 0 {
		t.Fatalf("git ls-files failed (exit=%d err=%v)", code, err)
	}
	for _, rel := range strings.Fields(untracked) {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		added[rel] = strings.Split(string(body), "\n")
	}
	return added
}

func isNarrativeComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "/*") {
		return false
	}
	return !strings.HasPrefix(trimmed, "//go:") && !strings.HasPrefix(trimmed, "// acs-predicate:")
}

func TestC2543_007_TheChangeAddsNoCodeComment(t *testing.T) {
	added := addedLines(t)
	if len(added) == 0 {
		t.Fatal("RED: the change adds no line under go/internal or go/cmd")
	}
	for rel, lines := range added {
		if !strings.HasSuffix(rel, ".go") {
			continue
		}
		for _, line := range lines {
			if isNarrativeComment(line) {
				t.Errorf("RED: %s adds a comment: %s", rel, strings.TrimSpace(line))
			}
		}
	}
}

func TestC2543_008_TheChangeEditsOnlyTestFilesInGo(t *testing.T) {
	for rel := range addedLines(t) {
		if strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go") {
			t.Errorf("RED: %s is production Go source; this item changes test files only", rel)
		}
	}
}

func TestC2543_009_EachChangedPackageNoteStatesTheHelperInSte(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range packageNotes {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		for _, needle := range []string{"gittest.IsolateHostCredentials", "TestMain"} {
			if !strings.Contains(string(body), needle) {
				t.Errorf("RED: %s does not state %q", rel, needle)
			}
		}
	}
	bin := filepath.Join(t.TempDir(), "evolve")
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "build", "-o", bin, "./cmd/evolve")
	if err != nil || code != 0 {
		t.Fatalf("go build ./cmd/evolve failed (exit=%d err=%v)\n%s\n%s", code, err, acsassert.OutputTail(stdout, 40), acsassert.OutputTail(stderr, 40))
	}
	stdout, stderr, code, err = acsassert.SubprocessOutput(bin, "docs", "ste-lint", "--strict", "--changed", originMain, "--added")
	if err != nil || code != 0 || !strings.Contains(stdout+stderr, "0 finding(s)") {
		t.Errorf("RED: ste-lint --strict on the added lines is not clean (exit=%d err=%v)\n%s\n%s", code, err, acsassert.OutputTail(stdout, 40), acsassert.OutputTail(stderr, 40))
	}
}

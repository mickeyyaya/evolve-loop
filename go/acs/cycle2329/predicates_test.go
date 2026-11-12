//go:build acs

package cycle2329

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	salvageBundleSuite = "TestApplyWorktrees_ASalvageRemovalStoresAVerifiedBundleOfTheLaneBranch"
	devBundleSuite     = "TestApplyDevWorktrees_(TheArchiveWrites|AClean|AnArchiveWhose)"
	scopedCoverRun     = "Salvage|Archive|ApplyWorktrees|ApplyDevWorktrees|DevArchive|Untracked|TarEntry"
	changedSourceDir   = "internal/gc"
)

var changedSources = []string{"salvage.go", "devarchive.go", "devworktrees_apply.go"}

func moduleDir(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := acsassert.ModuleDir(cwd)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func goInModule(t *testing.T, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = moduleDir(t)
	out, err := cmd.CombinedOutput()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("go %s: %v", strings.Join(args, " "), err)
	}
	return string(out), code
}

func runGCSuite(t *testing.T, pattern string, mustPass ...string) {
	t.Helper()
	out, code := goInModule(t, "test", "-count=1", "-v", "-run", "^"+pattern, "./"+changedSourceDir)
	if code != 0 {
		t.Fatalf("go test -run %s exit %d:\n%s", pattern, code, acsassert.OutputTail(out, 40))
	}
	for _, name := range mustPass {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("%s did not run and pass:\n%s", name, acsassert.OutputTail(out, 40))
		}
	}
}

func TestC2329_001_ASalvageWhoseBundleVerifyFailsKeepsTheTreeAndTheBranch(t *testing.T) {
	const suite = "TestApplyWorktrees_ASalvageWhoseBundleStepFailsKeepsTheTreeAndTheBranch"
	runGCSuite(t, suite, suite+"/verify", suite+"/create")
}

func TestC2329_002_ASalvageRemovalStoresAVerifiedBundleBesideHeadPatchAndUntracked(t *testing.T) {
	runGCSuite(t, salvageBundleSuite, salvageBundleSuite)
}

func TestC2329_003_TheDevArchiveWritesTheSameSalvageLayoutForDirtyAndCleanTrees(t *testing.T) {
	runGCSuite(t, devBundleSuite,
		"TestApplyDevWorktrees_TheArchiveWritesTheSalvageLeafWithAVerifiedBundle",
		"TestApplyDevWorktrees_ACleanTreeAlsoGetsAVerifiedBundleBeforeItsRemoval")
}

func TestC2329_004_ADevArchiveWhoseBundleStepFailsKeepsTheTreeTheBranchAndWritesNoRecord(t *testing.T) {
	const suite = "TestApplyDevWorktrees_AnArchiveWhoseBundleStepFailsKeepsTheTreeAndTheBranch"
	runGCSuite(t, suite, suite+"/verify", suite+"/create")
}

func declaredFuncs(t *testing.T, path string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			names = append(names, fn.Name.Name)
		}
	}
	return names
}

func TestC2329_005_EveryFunctionOfTheChangedSalvageAndArchiveFilesHasFullCoverage(t *testing.T) {
	scratch := t.TempDir()
	profile := filepath.Join(scratch, "cover.out")
	out, code := goInModule(t, "test", "-count=1", "-run", scopedCoverRun, "-coverprofile="+profile, "./"+changedSourceDir)
	if code != 0 {
		t.Fatalf("scoped gc tests exit %d:\n%s", code, acsassert.OutputTail(out, 40))
	}
	funcs, code := goInModule(t, "tool", "cover", "-func="+profile)
	if code != 0 {
		t.Fatalf("go tool cover exit %d:\n%s", code, funcs)
	}
	for _, source := range changedSources {
		names := declaredFuncs(t, filepath.Join(moduleDir(t), changedSourceDir, source))
		if len(names) == 0 {
			t.Fatalf("%s declares no function", source)
		}
		for _, name := range names {
			pct, err := acsassert.FuncCoverage(funcs, name)
			if err != nil {
				t.Errorf("%s: %v", source, err)
				continue
			}
			if pct != 100 {
				t.Errorf("%s %s covered %.1f%%, want 100%%", source, name, pct)
			}
		}
	}
}

func TestC2329_006_ThePackageNoteAndTheBuildExplanationNameDecisionsD1AndD2(t *testing.T) {
	root := acsassert.RepoRoot(t)
	note := filepath.Join(root, "docs", "architecture", "packages", "internal-gc.md")
	// acs-predicate: config-check
	if !acsassert.LineContainsAll(note, "D1", "refs.bundle") {
		t.Errorf("%s has no line that ties D1 to the refs.bundle of the salvage store", note)
	}
	// acs-predicate: config-check
	explanations, err := filepath.Glob(filepath.Join(root, "docs", "explain", "builds", "cycle-2329-*.md"))
	if err != nil || len(explanations) != 1 {
		t.Fatalf("want exactly one build explanation for cycle 2329, got %v (err %v)", explanations, err)
	}
	for _, id := range []string{"D1", "D2"} {
		if !acsassert.FileMatchesRegex(t, explanations[0], fmt.Sprintf(`\b%s\b`, id)) {
			t.Errorf("%s does not name decision %s", explanations[0], id)
		}
	}
}

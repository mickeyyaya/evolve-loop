package changedpkgs

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func shadowFixture(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	files := map[string]string{
		"go/go.mod":      "module example.com/m\n\ngo 1.23\n",
		"go/a/a.go":      "package a\n",
		"go/a/a_test.go": "package a\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n",
		"go/b/b.go":      "package b\n\nimport _ \"example.com/m/a\"\n",
		"go/b/b_test.go": "package b\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) {}\n",
		"go/d/d.go":      "package d\n\nimport _ \"example.com/m/b\"\n",
		"go/d/d_test.go": "package d\n\nimport \"testing\"\n\nfunc TestD(t *testing.T) {}\n",
		"go/c/c.go":      "package c\n",
		"go/c/c_test.go": "package c\n\nimport \"testing\"\n\nfunc TestC(t *testing.T) {}\n",
		"docs/x.md":      "x\n",
	}
	for rel, body := range files {
		full := filepath.Join(repo.Dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "baseline")
	return repo
}

func TestRunShadowTests_SelectsChangedPackageAndDirectAndTransitiveImporters(t *testing.T) {
	repo := shadowFixture(t)
	repo.Git("checkout", "-q", "-b", "lane")
	commitFile(t, repo, "go/a/a.go", "package a\n\nfunc A() {}\n")

	var res ShadowTestResult
	var err error
	res, err = RunShadowTests(context.Background(), repo.Dir, "", nil)
	if err != nil {
		t.Fatalf("RunShadowTests: %v", err)
	}
	sort.Strings(res.Selected)
	want := []string{"./a/...", "./b/...", "./d/..."}
	if !reflect.DeepEqual(res.Selected, want) {
		t.Errorf("Selected = %v, want %v", res.Selected, want)
	}
	if !res.Pass {
		t.Errorf("Pass = false, want true; output: %s", res.Output)
	}
	if res.BaseRef != "main" || res.MergeBase == "" {
		t.Errorf("BaseRef=%q MergeBase=%q, want main and non-empty SHA", res.BaseRef, res.MergeBase)
	}
}

func TestRunShadowTests_IgnoresMainCommitsAfterFork(t *testing.T) {
	repo := shadowFixture(t)
	repo.Git("checkout", "-q", "-b", "lane")
	repo.Git("checkout", "-q", "main")
	commitFile(t, repo, "go/c/c.go", "package c\n\nfunc C() {}\n")
	repo.Git("checkout", "-q", "lane")
	commitFile(t, repo, "go/a/a.go", "package a\n\nfunc A() {}\n")

	res, err := RunShadowTests(context.Background(), repo.Dir, "main", nil)
	if err != nil {
		t.Fatalf("RunShadowTests: %v", err)
	}
	sort.Strings(res.Selected)
	want := []string{"./a/...", "./b/...", "./d/..."}
	if !reflect.DeepEqual(res.Selected, want) {
		t.Errorf("Selected = %v, want %v (c changed on main only)", res.Selected, want)
	}
}

func TestRunShadowTests_NonGoChangeSelectsNothingAndPasses(t *testing.T) {
	repo := shadowFixture(t)
	repo.Git("checkout", "-q", "-b", "lane")
	commitFile(t, repo, "docs/x.md", "updated\n")

	called := false
	fake := func(_ context.Context, name, _ string, _ []string, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		if name == "go" {
			called = true
		}
		return 0, nil
	}

	res, err := RunShadowTests(context.Background(), repo.Dir, "main", fake)
	if err != nil {
		t.Fatalf("RunShadowTests: %v", err)
	}
	if called {
		t.Errorf("go test was invoked for a non-Go change")
	}
	if len(res.Selected) != 0 || !res.Pass {
		t.Errorf("got %+v, want empty Selected and Pass true", res)
	}
}

func TestRunShadowTests_FailingImporterTestReturnsPassFalse(t *testing.T) {
	repo := shadowFixture(t)
	repo.Git("checkout", "-q", "-b", "lane")
	commitFile(t, repo, "go/a/a.go", "package a\n\nfunc A() {}\n")
	commitFile(t, repo, "go/b/b_test.go", "package b\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) { t.Fatal(\"boom\") }\n")

	res, err := RunShadowTests(context.Background(), repo.Dir, "main", nil)
	if err != nil {
		t.Fatalf("RunShadowTests: %v", err)
	}
	if res.Pass {
		t.Errorf("Pass = true, want false when importer test fails")
	}
	if !strings.Contains(res.Output, "FAIL") {
		t.Errorf("Output = %q, want FAIL", res.Output)
	}
}

func TestRunShadowTests_ErrorsOnInvalidInputsAndRunnerFailures(t *testing.T) {
	if _, err := RunShadowTests(context.Background(), "", "main", nil); err == nil {
		t.Errorf("expected error for empty repoRoot")
	}
	if _, err := RunShadowTests(context.Background(), t.TempDir(), "main", nil); err == nil {
		t.Errorf("expected error for non-git directory")
	}
	repo := shadowFixture(t)
	if _, err := RunShadowTests(context.Background(), repo.Dir, "no-such-ref", nil); err == nil {
		t.Errorf("expected error for unknown baseRef")
	}

	repo.Git("checkout", "-q", "-b", "lane")
	commitFile(t, repo, "go/a/a.go", "package a\n\nfunc A() {}\n")
	errRunner := func(_ context.Context, name, _ string, _ []string, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		if name == "go" {
			return -1, errors.New("exec failed")
		}
		return 0, nil
	}
	if _, err := RunShadowTests(context.Background(), repo.Dir, "main", errRunner); err == nil {
		t.Errorf("expected error when go test runner fails to execute")
	}

	noModRepo := gittest.Fixture(t)
	commitFile(t, noModRepo, "docs/x.md", "x\n")
	noModRepo.Git("checkout", "-q", "-b", "lane")
	commitFile(t, noModRepo, "go/a/a.go", "package a\n")
	if _, err := RunShadowTests(context.Background(), noModRepo.Dir, "main", nil); err == nil {
		t.Errorf("expected error when go.mod is missing for importer closure")
	}

	corruptRepo := shadowFixture(t)
	treeSHA := strings.TrimSpace(corruptRepo.Git("rev-parse", "main^{tree}"))
	corruptRepo.Git("checkout", "-q", "-b", "lane")
	objPath := filepath.Join(corruptRepo.Dir, ".git", "objects", treeSHA[:2], treeSHA[2:])
	if err := os.Remove(objPath); err != nil {
		t.Fatalf("remove tree object: %v", err)
	}
	if _, err := RunShadowTests(context.Background(), corruptRepo.Dir, "main", nil); err == nil {
		t.Errorf("expected error when merge-base tree object is unreadable")
	}
}

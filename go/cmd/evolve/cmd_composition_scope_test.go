package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const scopeRecordingMakefile = "build:\n\t@echo ran > compile.ran\n" +
	"test:\n\t@printf '%s' '$(FAST_PKGS)' > test.scope\n" +
	"test-acs-durable:\n\t@echo ran > acs.ran\n" +
	"apicover-enforce:\n\t@printf '%s' '$(APICOVER_PKGS)' > apicover.scope\n"

func scopeWorktree(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	files := map[string]string{
		"go/go.mod":            "module example.com/m\n\ngo 1.23\n",
		"go/Makefile":          scopeRecordingMakefile,
		"go/a/a.go":            "package a\n",
		"go/b/b.go":            "package b\n\nimport _ \"example.com/m/a\"\n",
		"go/c/c.go":            "package c\n",
		"go/.apicover-enforce": "# enforced\n\n./a\n./b\n./c\n",
		"docs/x.md":            "x\n",
	}
	for rel, body := range files {
		writeScopeFile(t, repo.Dir, rel, body)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "baseline")
	repo.Git("checkout", "-q", "-b", "lane")
	return repo
}

func writeScopeFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitScopeChange(t *testing.T, repo *gittest.Repo, rel, body string) {
	t.Helper()
	writeScopeFile(t, repo.Dir, rel, body)
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "lane change")
}

func recorded(t *testing.T, dir, name string) (string, bool) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "go", name))
	if os.IsNotExist(err) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(raw), true
}

func TestComposedGateScope_TestGateRunsOnlyChangedAndImporters(t *testing.T) {
	t.Parallel()
	repo := scopeWorktree(t)
	commitScopeChange(t, repo, "go/a/a.go", "package a\n\nfunc A() {}\n")
	var log bytes.Buffer

	results := composedGatesTo(&log)(context.Background(), repo.Dir)

	if results["test"].Status != "pass" {
		t.Fatalf("results[test] = %#v, want pass", results["test"])
	}
	got, ran := recorded(t, repo.Dir, "test.scope")
	if !ran || got != "./a/... ./b/..." {
		t.Fatalf("test gate ran=%v with FAST_PKGS=%q, want exactly %q: c does not import a", ran, got, "./a/... ./b/...")
	}
}

func TestComposedGateScope_ApicoverGateRunsOnlyEnforcedEntriesInTheClosure(t *testing.T) {
	t.Parallel()
	repo := scopeWorktree(t)
	commitScopeChange(t, repo, "go/a/a.go", "package a\n\nfunc A() {}\n")

	results := composedGatesTo(&bytes.Buffer{})(context.Background(), repo.Dir)

	if results["apicover"].Status != "pass" {
		t.Fatalf("results[apicover] = %#v, want pass", results["apicover"])
	}
	got, ran := recorded(t, repo.Dir, "apicover.scope")
	if !ran || got != "./a ./b" {
		t.Fatalf("apicover gate ran=%v with APICOVER_PKGS=%q, want exactly %q", ran, got, "./a ./b")
	}
}

func TestComposedGateScope_UnderivableChangeRunsTheFullSet(t *testing.T) {
	t.Parallel()
	worktree := t.TempDir()
	writeScopeFile(t, worktree, "go/Makefile", scopeRecordingMakefile)

	results := composedGatesTo(&bytes.Buffer{})(context.Background(), worktree)

	for _, gate := range []string{"test", "apicover"} {
		if results[gate].Status != "pass" {
			t.Fatalf("results[%s] = %#v, want pass", gate, results[gate])
		}
	}
	if got, ran := recorded(t, worktree, "test.scope"); !ran || got != "" {
		t.Errorf("test gate ran=%v with FAST_PKGS=%q, want the whole-module run with no override", ran, got)
	}
	if got, ran := recorded(t, worktree, "apicover.scope"); !ran || got != "" {
		t.Errorf("apicover gate ran=%v with APICOVER_PKGS=%q, want the whole-module run with no override", ran, got)
	}
}

func TestComposedGateScope_ModuleRootChangeRunsTheFullSet(t *testing.T) {
	t.Parallel()
	repo := scopeWorktree(t)
	commitScopeChange(t, repo, "go/go.mod", "module example.com/m\n\ngo 1.23\n\n")

	composedGatesTo(&bytes.Buffer{})(context.Background(), repo.Dir)

	if got, ran := recorded(t, repo.Dir, "test.scope"); !ran || got != "" {
		t.Errorf("test gate ran=%v with FAST_PKGS=%q, want the whole-module run", ran, got)
	}
}

func TestComposedGateScope_ChangeOutsideGoSkipsTheScopedGatesAndSaysSo(t *testing.T) {
	t.Parallel()
	repo := scopeWorktree(t)
	commitScopeChange(t, repo, "docs/x.md", "changed\n")
	var log bytes.Buffer

	results := composedGatesTo(&log)(context.Background(), repo.Dir)

	for _, gate := range []string{"test", "apicover"} {
		if results[gate].Status != "pass" {
			t.Errorf("results[%s] = %#v, want pass without a run", gate, results[gate])
		}
	}
	if _, ran := recorded(t, repo.Dir, "test.scope"); ran {
		t.Error("test gate ran make although no Go file changed: an empty list would test the current directory")
	}
	if _, ran := recorded(t, repo.Dir, "apicover.scope"); ran {
		t.Error("apicover gate ran make although no Go file changed")
	}
	if !strings.Contains(log.String(), "test") {
		t.Errorf("log = %q, want one line that names the skipped test gate", log.String())
	}
}

func TestComposedGateScope_CompileAndAcsKeepTheWholeModuleTargets(t *testing.T) {
	t.Parallel()
	repo := scopeWorktree(t)
	commitScopeChange(t, repo, "docs/x.md", "changed\n")

	results := composedGatesTo(&bytes.Buffer{})(context.Background(), repo.Dir)

	for _, gate := range []string{"compile", "acs"} {
		if results[gate].Status != "pass" {
			t.Errorf("results[%s] = %#v, want pass", gate, results[gate])
		}
	}
	for _, marker := range []string{"compile.ran", "acs.ran"} {
		if _, ran := recorded(t, repo.Dir, marker); !ran {
			t.Errorf("%s missing: the gate has no package scope and must always run", marker)
		}
	}
}

func TestComposedGateScope_MissingEnforceListRunsTheFullApicoverSet(t *testing.T) {
	t.Parallel()
	repo := scopeWorktree(t)
	repo.Git("rm", "-q", "go/.apicover-enforce")
	repo.Git("commit", "-q", "-m", "drop list")
	commitScopeChange(t, repo, "go/a/a.go", "package a\n\nfunc A() {}\n")

	composedGatesTo(&bytes.Buffer{})(context.Background(), repo.Dir)

	if got, ran := recorded(t, repo.Dir, "apicover.scope"); !ran || got != "" {
		t.Errorf("apicover gate ran=%v with APICOVER_PKGS=%q, want the whole-module run when the list cannot be read", ran, got)
	}
}

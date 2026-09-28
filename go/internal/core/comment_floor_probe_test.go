package core

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func floorRepo(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n\nfunc a() {}\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	return repo, repo.Git("rev-parse", "HEAD")
}

func floorRoot(t *testing.T, policyJSON string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".evolve", "policy.json"), policyJSON)
	return root
}

func floorInput(t *testing.T, root string, repo *gittest.Repo, base string) ReviewInput {
	t.Helper()
	return ReviewInput{Phase: string(PhaseBuild), ProjectRoot: root, Worktree: repo.Dir, WorktreeBaseSHA: base}
}

func TestCommentFloorFailures_ACommittedBuildIsJudgedAgainstItsBase(t *testing.T) {
	repo, base := floorRepo(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n\n// restates the helper.\nfunc a() {}\n")
	repo.Git("commit", "-q", "-am", "build")

	failures := commentFloorFailures(context.Background(), floorInput(t, floorRoot(t, `{"comment_floor":{"stage":"enforce"}}`), repo, base))

	if len(failures) != 1 || !strings.Contains(failures[0], "go/p/a.go: // restates the helper.") {
		t.Fatalf("a committed build's added comment must be named: %v", failures)
	}
}

func TestCommentFloorFailures_WithoutAPolicyBlockItOnlyShadows(t *testing.T) {
	repo, base := floorRepo(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n\n// restates the helper.\nfunc a() {}\n")

	if failures := commentFloorFailures(context.Background(), floorInput(t, t.TempDir(), repo, base)); failures != nil {
		t.Fatalf("with no comment_floor block the floor shadows, got failures %v", failures)
	}
}

func TestCommentFloorFailures_ANewFilesPackageDocIsNotAdded(t *testing.T) {
	repo, base := floorRepo(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "q", "q.go"), "// Package q holds the fixture for the floor.\npackage q\n")

	if failures := commentFloorFailures(context.Background(), floorInput(t, floorRoot(t, `{"comment_floor":{"stage":"enforce"}}`), repo, base)); failures != nil {
		t.Fatalf("a new file's package doc is spared, got %v", failures)
	}
}

func TestCommentFloorFailures_AnUnreadableDiffFailsOpen(t *testing.T) {
	repo, base := floorRepo(t)
	if err := os.MkdirAll(filepath.Join(repo.Dir, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(repo.Dir, "d"), filepath.Join(repo.Dir, "go", "p", "dir.go")); err != nil {
		t.Fatal(err)
	}

	if failures := commentFloorFailures(context.Background(), floorInput(t, floorRoot(t, `{"comment_floor":{"stage":"enforce"}}`), repo, base)); failures != nil {
		t.Fatalf("an unreadable diff fails open with a WARN, got %v", failures)
	}
}

func TestCommentFloorStage_AnUnknownWordIsOff(t *testing.T) {
	for word, want := range map[string]config.Stage{"enforce": config.StageEnforce, " enforce ": config.StageEnforce, "shadow": config.StageShadow, "enforced": config.StageOff, "Enforce": config.StageOff} {
		if got := commentFloorStage(floorRoot(t, `{"comment_floor":{"stage":"`+word+`"}}`)); got != want {
			t.Errorf("stage %q = %s, want %s", word, got, want)
		}
	}
	if got := commentFloorStage(""); got != config.StageShadow {
		t.Errorf("no project root = %s, want the compiled default shadow", got)
	}
}

func TestWorktreeGit_CarriesGitsOwnErrorInsteadOfPrintingIt(t *testing.T) {
	repo, _ := floorRepo(t)

	_, err := worktreeGit(context.Background(), repo.Dir)("cat-file", "-e", "HEAD:go/p/missing.go")

	if err == nil || !strings.Contains(err.Error(), "fatal:") {
		t.Fatalf("err = %v, want git's own fatal message carried in the error", err)
	}
}

func TestChangedWorktreePathsSince_NamesANonASCIIPathAsGitStoresIt(t *testing.T) {
	repo, base := floorRepo(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "café.go"), "package p\n")
	repo.Git("add", "go/p/café.go")
	repo.Git("commit", "-q", "-m", "a tracked non-ASCII name")
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "café.go"), "package p\n\nfunc c() {}\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "naïve.go"), "package p\n")

	got := changedWorktreePathsSince(context.Background(), repo.Dir, base)

	if want := []string{"go/p/café.go", "go/p/naïve.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("paths = %q, want %q", got, want)
	}
}

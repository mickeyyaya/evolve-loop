package changeloggen

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initGitRepo materializes a temp git repo with a known commit history
// for ReadGitLog + VerifyRef coverage. Uses a hermetic env so the test
// doesn't pick up the user's git config / signing keys.
func initGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	tmp := t.TempDir()
	mustRun := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmp
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	mustRun("init", "-q", "-b", "main")
	// Seed two commits + tag the first.
	if err := os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("seed a: %v", err)
	}
	mustRun("add", "a.txt")
	mustRun("commit", "-m", "feat: first thing")
	mustRun("tag", "v0.1.0")
	if err := os.WriteFile(filepath.Join(tmp, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatalf("seed b: %v", err)
	}
	mustRun("add", "b.txt")
	mustRun("commit", "-m", "fix: second thing")
	if err := os.WriteFile(filepath.Join(tmp, "c.txt"), []byte("c"), 0o644); err != nil {
		t.Fatalf("seed c: %v", err)
	}
	mustRun("add", "c.txt")
	mustRun("commit", "-m", "chore: skip me")
	return tmp
}

func TestReadGitLog_RealRepo(t *testing.T) {
	repo := initGitRepo(t)
	commits, err := ReadGitLog(repo, "v0.1.0", "HEAD")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits since v0.1.0, got %d: %+v", len(commits), commits)
	}
	// Most recent commit first (git log default).
	if !strings.HasPrefix(commits[0].Subject, "chore:") {
		t.Errorf("first commit subject=%q", commits[0].Subject)
	}
	if !strings.HasPrefix(commits[1].Subject, "fix:") {
		t.Errorf("second commit subject=%q", commits[1].Subject)
	}
	for _, c := range commits {
		if len(c.SHA) != 40 {
			t.Errorf("SHA wrong length: %q", c.SHA)
		}
	}
}

func TestReadGitLog_NoCommitsInRange(t *testing.T) {
	repo := initGitRepo(t)
	// HEAD..HEAD should be empty.
	_, err := ReadGitLog(repo, "HEAD", "HEAD")
	if !errors.Is(err, ErrNoCommits) {
		t.Errorf("expected ErrNoCommits, got %v", err)
	}
}

func TestReadGitLog_InvalidRefSurfacesError(t *testing.T) {
	repo := initGitRepo(t)
	_, err := ReadGitLog(repo, "v99.99.99", "HEAD")
	if err == nil {
		t.Errorf("expected error for invalid ref")
	}
	if errors.Is(err, ErrNoCommits) {
		t.Errorf("invalid ref should not be ErrNoCommits")
	}
}

func TestVerifyRef_ValidRefOK(t *testing.T) {
	repo := initGitRepo(t)
	if err := VerifyRef(repo, "v0.1.0"); err != nil {
		t.Errorf("v0.1.0 should resolve: %v", err)
	}
	if err := VerifyRef(repo, "HEAD"); err != nil {
		t.Errorf("HEAD should resolve: %v", err)
	}
}

func TestVerifyRef_InvalidRefError(t *testing.T) {
	repo := initGitRepo(t)
	err := VerifyRef(repo, "nonexistent-tag")
	if err == nil || !strings.Contains(err.Error(), "ref does not exist") {
		t.Errorf("got %v", err)
	}
}

func TestWriteEntry_ReadError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root cannot mask read")
	}
	tmp := t.TempDir()
	cl := filepath.Join(tmp, "CHANGELOG.md")
	_ = os.WriteFile(cl, []byte("body"), 0o000)
	t.Cleanup(func() { _ = os.Chmod(cl, 0o644) })
	_, _, err := WriteEntry(cl, "1.0.0", "## [1.0.0]\n")
	if err == nil {
		t.Errorf("expected read error")
	}
}

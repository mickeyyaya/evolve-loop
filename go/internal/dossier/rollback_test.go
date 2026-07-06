package dossier

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

// rollback_test.go — RED contract for cycle-573 Task 3
// (dossier-commit-rollback-on-failure, inbox weight 0.84 medium).
//
// commitPairGit stages cycle-<base>.{json,md} via `git add`, then commits. On a
// PERMANENT (non-lock) commit failure it returns the error but never unstages
// the pair — so the staged files survive into the next cycle's tree-diff guard
// as phantom staged changes. The fix: on a permanent failure, `git reset` the
// pair back out of the index before returning the (unchanged) error.
//
// The load-bearing invariant is the STAGED set, not the whole porcelain: newly
// created files legitimately remain on disk as untracked after an unstage; the
// pollution the guard trips on is a non-empty index. So the assertion is
// `git diff --cached --name-only` == empty.
//
// RED today: after the failed commit the pair is still staged, so the staged set
// is non-empty. GREEN once commitPairGit resets on permanent failure.

// stagedPaths returns the names in the git index (staged set) for dir.
func stagedPaths(t *testing.T, dir string) []string {
	t.Helper()
	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git diff --cached: %v\n%s", err, out)
	}
	var paths []string
	for _, ln := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			paths = append(paths, ln)
		}
	}
	return paths
}

// TestCommitPairGit_RollsBackStagedOnPermanentFailure — AC-3a (behavioural):
// with an identity-less git env (a permanent, non-lock commit failure),
// commitPairGit must (1) still return the underlying error and (2) leave the
// index empty — no staged pair to pollute the next cycle's tree-diff guard.
func TestCommitPairGit_RollsBackStagedOnPermanentFailure(t *testing.T) {
	// Isolate git config so the run never touches the host's config.
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	dir := t.TempDir()
	// Baseline commit WITH a one-shot inline identity so HEAD exists (git reset --
	// <path> needs a resolvable HEAD). The identity is never persisted to config.
	for _, args := range [][]string{
		{"init"},
		{"-c", "user.email=seed@example.com", "-c", "user.name=seed", "commit", "--allow-empty", "-m", "baseline"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// Force a PERMANENT (non-lock) commit failure deterministically on any host:
	// an explicitly-empty author/committer ident makes `git commit` fatal
	// ("empty ident name not allowed"), regardless of gecos/hostname auto-detect.
	// Set only now — after the baseline commit — so commitPairGit inherits it via
	// os.Environ() but the baseline above still succeeds.
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "")
	}

	base := "cycle-573"
	for _, name := range []string{base + ".json", base + ".md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	err := commitPairGit(gitexec.Default(dir), base)
	if err == nil {
		t.Fatalf("commitPairGit: expected a permanent commit failure, got nil")
	}

	if staged := stagedPaths(t, dir); len(staged) != 0 {
		t.Errorf("staged pair not rolled back after permanent commit failure: index still holds %v", staged)
	}
}

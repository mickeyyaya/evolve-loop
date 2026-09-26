package dossier

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

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

func TestCommitPairGit_RollsBackStagedOnPermanentFailure(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	dir := t.TempDir()
	// A baseline commit gives `git reset -- <path>` a resolvable HEAD.
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

	// An empty ident makes `git commit` fail permanently on any host. Set after the
	// baseline commit, so only commitPairGit inherits it.
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

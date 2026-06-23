package swarm_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/swarm"
)

func TestGitProvisioner_RelativeWorktreeBaseHasNoFilesystemSideEffects(t *testing.T) {
	repo := initAmplificationGitRepo(t)
	cwd := t.TempDir()
	chdirForAmplification(t, cwd)

	relativeBase := filepath.Join("relative-base", "nested")
	t.Setenv("EVOLVE_WORKTREE_BASE", relativeBase)

	got, err := swarm.NewGitWorkerProvisioner(nil).CreateIntegration(context.Background(), repo, 294)
	if err == nil {
		t.Fatalf("CreateIntegration succeeded with relative EVOLVE_WORKTREE_BASE; path=%q", got)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "absolute") {
		t.Fatalf("CreateIntegration error = %q, want mention of absolute worktree base", err)
	}
	if got != "" {
		t.Fatalf("CreateIntegration returned path %q on rejected relative base, want empty path", got)
	}

	assertMissingAmplificationPath(t, filepath.Join(cwd, relativeBase))
	assertMissingAmplificationPath(t, filepath.Join(repo, relativeBase))
}

// chdirForAmplification is testing.T.Chdir for the CI-pinned toolchain —
// t.Chdir is Go 1.24+; CI runs Go 1.23. os.Chdir is process-global: must
// not be called from a t.Parallel test.
func chdirForAmplification(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(prev); err != nil {
			t.Errorf("restore cwd %s: %v", prev, err)
		}
	})
}

func initAmplificationGitRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	runAmplificationGit(t, repo, "init")
	runAmplificationGit(t, repo, "config", "user.email", "amplification@example.invalid")
	runAmplificationGit(t, repo, "config", "user.name", "Amplification Test")

	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("amplification fixture\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	runAmplificationGit(t, repo, "add", "README.md")
	runAmplificationGit(t, repo, "commit", "-m", "initial")
	return repo
}

func runAmplificationGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func assertMissingAmplificationPath(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		t.Fatalf("relative worktree base side effect exists at %s", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

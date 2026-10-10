//go:build integration

package gc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestPlanAndApplyWorktrees_RealGit_MergedWorktreeIsSweptEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-q", "-m", "init")

	base := filepath.Join(root, ".evolve", "worktrees")
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	branch := "cycle-realgit1-800"
	wt := filepath.Join(base, branch)
	runGit(t, root, "worktree", "add", "-B", branch, wt, "HEAD")

	opts := WorktreeOptions{
		ProjectRoot:  root,
		WorktreeBase: base,
		EvolveDir:    evolveDir,
		Policy:       WorktreesPolicy{KeepRecent: 0, MinAgeMinutes: 0},
		Now:          func() time.Time { return time.Now().Add(24 * time.Hour) },
		Exec:         sysexec.DefaultRunner,
		LeaseTTL:     10 * time.Minute,
	}

	m, err := PlanWorktrees(opts)
	if err != nil {
		t.Fatalf("PlanWorktrees: %v", err)
	}
	foundRemove, foundDelete := false, false
	for _, it := range m.Items {
		if it.Action == WorktreeActionRemove && it.Path == wt {
			foundRemove = true
		}
		if it.Action == WorktreeActionDeleteBranch && it.Branch == branch {
			foundDelete = true
		}
	}
	if !foundRemove || !foundDelete {
		t.Fatalf("expected remove+delete-branch for the merged clean worktree, got %+v", m.Items)
	}

	if err := ApplyWorktrees(opts, m); err != nil {
		t.Fatalf("ApplyWorktrees: %v", err)
	}

	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("worktree dir must be gone after Apply: stat err=%v", err)
	}
	listOut := runGit(t, root, "worktree", "list", "--porcelain")
	if strings.Contains(listOut, wt) {
		t.Errorf("git worktree list must no longer reference %s:\n%s", wt, listOut)
	}
	branchOut := runGit(t, root, "branch", "--list", branch)
	if strings.TrimSpace(branchOut) != "" {
		t.Errorf("branch %s must be deleted, git branch --list still shows: %q", branch, branchOut)
	}
}

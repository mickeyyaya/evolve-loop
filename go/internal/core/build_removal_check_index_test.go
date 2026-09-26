package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitInIndex uses `git -C` so the fixture repo never resolves from the process cwd.
func gitInIndex(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	// Hermetic: a developer's global config (gpgsign=true) must not reach the fixture repo.
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=cycle1591", "GIT_AUTHOR_EMAIL=cycle1591@example.invalid",
		"GIT_COMMITTER_NAME=cycle1591", "GIT_COMMITTER_EMAIL=cycle1591@example.invalid")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func trackedThenDeleted(t *testing.T, claimed string) ReviewInput {
	t.Helper()
	in := removalFixture(t, claimBlock(claimed), []string{claimed})
	gitInIndex(t, in.Worktree, "init", "-q")
	gitInIndex(t, in.Worktree, "add", "--", claimed)
	gitInIndex(t, in.Worktree, "commit", "-q", "-m", "track the record")
	if err := os.Remove(filepath.Join(in.Worktree, claimed)); err != nil {
		t.Fatalf("remove worktree copy: %v", err)
	}
	return in
}

func TestRemovalClaimFailures_TrackedButAbsentFromDisk(t *testing.T) {
	const claimed = ".evolve/inbox/2026-08-18T02-30-00Z-retro-prompt-delivery-stall.json"
	in := trackedThenDeleted(t, claimed)

	got := RemovalClaimFailures(context.Background(), in)
	if len(got) != 1 {
		t.Fatalf("tracked-but-absent claim failures = %d, want 1 (a filesystem-only "+
			"retirement is undone by the next checkout): %v", len(got), got)
	}
	if !strings.Contains(got[0], claimed) {
		t.Fatalf("failure does not name the claimed path %q: %v", claimed, got)
	}
}

func TestRemovalClaimFailures_UntrackedAbsent_StaysHonest(t *testing.T) {
	const claimed = "go/acs/cycle1591/scratch.txt"

	t.Run("untracked and absent in a real repo — honest", func(t *testing.T) {
		in := removalFixture(t, claimBlock(claimed), nil)
		gitInIndex(t, in.Worktree, "init", "-q")
		if got := RemovalClaimFailures(context.Background(), in); len(got) != 0 {
			t.Fatalf("untracked absent path must remain an honest removal; got %v", got)
		}
	})

	t.Run("worktree is not a git repo — fail open", func(t *testing.T) {
		in := removalFixture(t, claimBlock(claimed), nil)
		if got := RemovalClaimFailures(context.Background(), in); len(got) != 0 {
			t.Fatalf("a non-repo worktree must fail open (nil), never block; got %v", got)
		}
	})
}

func TestRemovalClaimFailures_TrackedAndPresent_ReportsExactlyOnce(t *testing.T) {
	const claimed = ".evolve/inbox/still-here.json"
	in := removalFixture(t, claimBlock(claimed), []string{claimed})
	gitInIndex(t, in.Worktree, "init", "-q")
	gitInIndex(t, in.Worktree, "add", "--", claimed)

	got := RemovalClaimFailures(context.Background(), in)
	if len(got) != 1 {
		t.Fatalf("a claim that is false on BOTH axes must produce exactly 1 failure, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], "still exists in the worktree") {
		t.Fatalf("on-disk false claim must keep the cycle-660 message; got %q", got[0])
	}
}

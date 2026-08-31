package core

// build_removal_check_index_test.go — RED contract for the cycle-1591 task
// `retire-stale-retro-prompt-delivery-stall`.
//
// The incident: the live inbox record
// `.evolve/inbox/2026-08-18T02-30-00Z-retro-prompt-delivery-stall.json` has been
// "retired" more than once by a filesystem-only removal — a plain delete, or a
// move into the .gitignore'd `.evolve/inbox/processed/` destination — with no
// matching `git rm`. The path stayed in the Git INDEX, so the next fresh
// checkout restored it and the item reopened as live, burning an empty lane.
//
// `RemovalClaimFailures` is the build-floor gate that exists to catch a false
// "I removed this" claim, but it asks only the worktree filesystem
// (`os.Stat`, build_removal_check.go:60-62). A path absent from disk yet still
// tracked reads to it as an honest removal, so the false retirement passed the
// floor. The claim is about the state of the REPOSITORY, not of one working
// tree, so the index is the second half of the truth check.
//
// Contract under test (production change is Builder's job — none of it exists
// at RED time): when a claimed path is absent from the worktree filesystem but
// still present in that worktree's Git index, RemovalClaimFailures must return
// exactly one failure naming the path. Every existing disposition is preserved:
// an untracked absent path is still an honest removal, a non-repo worktree
// still fails open, and a path still on disk still produces exactly one
// failure (never two).

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitInIndex runs git inside dir, failing the test on error. `git -C` (never a
// bare `git` resolved from process cwd) so the fixture repo is unambiguous
// under fleet lanes and worktrees.
func gitInIndex(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	// Hermetic: no ambient identity, hooks, or signing config may reach the
	// fixture repo (a developer's global gpgsign=true would fail `git add`'s
	// sibling operations and make this test machine-dependent).
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=cycle1591", "GIT_AUTHOR_EMAIL=cycle1591@example.invalid",
		"GIT_COMMITTER_NAME=cycle1591", "GIT_COMMITTER_EMAIL=cycle1591@example.invalid")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// trackedThenDeleted builds the exact incident shape: a claimed path that is
// committed to the fixture worktree's index and then removed from disk ONLY.
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

// AC1 (the defect): absent from disk, still in the index — a FALSE removal
// claim, because a fresh checkout of this ref restores the file.
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

// AC2 (NEGATIVE — the anti-overreach guard): the fix must consult the index,
// not merely start failing every absent path. A genuinely untracked file that
// is gone from disk is an HONEST removal and must stay silent; a worktree that
// is not a Git repository at all must still fail open (the floor never
// false-blocks a build over its own plumbing).
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

// AC3 (edge — no double-counting): a path that is BOTH still on disk and still
// tracked is one false claim, not two. The cycle-660 message stays the one the
// operator reads for the on-disk case.
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

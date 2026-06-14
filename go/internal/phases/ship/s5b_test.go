//go:build integration

package ship

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// divergedWorktree sets up the concurrency case: a cycle worktree branch created
// at main, plus a peer commit advancing main independently, so the cycle branch
// can no longer fast-forward into main. Returns the worktree path.
func divergedWorktree(t *testing.T, repo string) string {
	t.Helper()
	// Force the base branch to "main": makeRepo does `git init` without -b main,
	// so the default branch name is host-dependent (main locally, master on CI).
	// (The existing happy-path tests get "main" via addRemote, which this test
	// doesn't need — the ff-merge diverges before any push.)
	runGit(t, repo, "branch", "-M", "main")
	wt := makeWorktree(t, repo, "cycle-1-branch") // branch at current main (M0)
	mustWrite(t, filepath.Join(wt, "feature.txt"), "this cycle's change\n")
	// A peer cycle moves main forward (M0 -> M1) on a DISJOINT file.
	mustWrite(t, filepath.Join(repo, "peer.txt"), "a peer cycle's change\n")
	runGit(t, repo, "add", "peer.txt")
	runGit(t, repo, "commit", "-m", "peer cycle moved main")
	return wt
}

func shipFromWorktreeOpts(repo string, env map[string]string) *Options {
	return &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: this cycle",
		ProjectRoot:   repo,
		PluginRoot:    repo,
		Runner:        execRunner,
		Stdin:         strings.NewReader(""),
		Stdout:        io.Discard,
		Stderr:        io.Discard,
		Env:           env,
	}
}

// TestShipFromWorktree_FleetMode_DivergedFFMerge_SignalsRebase pins ADR-0049 S5b:
// under fleet mode a ff-merge divergence (a peer cycle moved main) is the
// EXPECTED concurrency case, not a terminal failure — ship signals
// GIT_FLEET_REBASE_NEEDED (transient) so the orchestrator rebases + re-verifies
// the merged tree and re-ships. RED before the fleet branch (returns the
// terminal GIT_FF_MERGE_DIVERGED), GREEN after.
func TestShipFromWorktree_FleetMode_DivergedFFMerge_SignalsRebase(t *testing.T) {
	repo := makeRepo(t)
	wt := divergedWorktree(t, repo)
	opts := shipFromWorktreeOpts(repo, map[string]string{"EVOLVE_FLEET": "1"})

	err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt)
	var se *core.ShipError
	if err == nil || !errors.As(err, &se) || se.Code != core.CodeGitFleetRebaseNeeded {
		t.Fatalf("fleet diverged ff-merge: want GIT_FLEET_REBASE_NEEDED, got %v", err)
	}
}

// TestShipFromWorktree_NonFleet_DivergedFFMerge_Terminal: the sequential loop is
// unchanged — a diverged ff-merge is still the terminal GIT_FF_MERGE_DIVERGED.
func TestShipFromWorktree_NonFleet_DivergedFFMerge_Terminal(t *testing.T) {
	repo := makeRepo(t)
	wt := divergedWorktree(t, repo)
	opts := shipFromWorktreeOpts(repo, nil) // no EVOLVE_FLEET

	err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt)
	var se *core.ShipError
	if err == nil || !errors.As(err, &se) || se.Code != core.CodeGitFFMergeDiverged {
		t.Fatalf("non-fleet diverged ff-merge: want GIT_FF_MERGE_DIVERGED, got %v", err)
	}
}

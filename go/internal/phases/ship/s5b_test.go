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

func divergedWorktree(t *testing.T, repo string) string {
	t.Helper()
	runGit(t, repo, "branch", "-M", "main")
	wt := makeWorktree(t, repo, "cycle-1-branch")
	mustWrite(t, filepath.Join(wt, "feature.txt"), "this cycle's change\n")
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

func TestShipFromWorktree_FleetMode_DivergedFFMerge_SignalsRebase(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	wt := divergedWorktree(t, repo)
	opts := shipFromWorktreeOpts(repo, map[string]string{"EVOLVE_FLEET": "1"})

	err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt)
	var se *core.ShipError
	if err == nil || !errors.As(err, &se) || se.Code != core.CodeGitFleetRebaseNeeded {
		t.Fatalf("fleet diverged ff-merge: want GIT_FLEET_REBASE_NEEDED, got %v", err)
	}
}

func TestShipFromWorktree_NonFleet_DivergedFFMerge_Terminal(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	wt := divergedWorktree(t, repo)
	opts := shipFromWorktreeOpts(repo, nil)

	err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt)
	var se *core.ShipError
	if err == nil || !errors.As(err, &se) || se.Code != core.CodeGitFFMergeDiverged {
		t.Fatalf("non-fleet diverged ff-merge: want GIT_FF_MERGE_DIVERGED, got %v", err)
	}
}

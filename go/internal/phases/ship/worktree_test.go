//go:build integration

package ship

import (
	"path/filepath"
	"testing"
)

func TestShipFromWorktree_AcquiresAndReleasesShipLock(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "cycle-1-branch")
	mustWrite(t, filepath.Join(wt, "feature.txt"), "worktree feature\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	seedAudit(t, repo, "PASS")

	var acquired, released int
	var lockedPath string
	opts := Options{
		Class:         ClassCycle,
		CommitMessage: "feat: worktree ship",
		shipLock: func(path string) (func(), error) {
			acquired++
			lockedPath = path
			return func() { released++ }, nil
		},
	}
	res, err := runShip(t, repo, opts)
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK, got %d (err=%v logs=%v)", res.ExitCode, err, res.Logs)
	}
	if acquired != 1 || released != 1 {
		t.Fatalf("ship lock acquired=%d released=%d, want 1/1", acquired, released)
	}
	if filepath.Base(lockedPath) != "ship.lock" {
		t.Errorf("locked %q, want a path ending in ship.lock", lockedPath)
	}
}

func TestShipFromWorktree_DryRun_SkipsShipLock(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "cycle-1-branch")
	mustWrite(t, filepath.Join(wt, "feature.txt"), "worktree feature\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	seedAudit(t, repo, "PASS")

	var acquired int
	opts := Options{
		Class:         ClassCycle,
		DryRun:        true,
		CommitMessage: "feat: worktree ship",
		shipLock:      func(string) (func(), error) { acquired++; return func() {}, nil },
	}
	if _, err := runShip(t, repo, opts); err != nil {
		t.Fatalf("dry-run ship: %v", err)
	}
	if acquired != 0 {
		t.Errorf("dry-run must not acquire the ship lock; acquired=%d", acquired)
	}
}

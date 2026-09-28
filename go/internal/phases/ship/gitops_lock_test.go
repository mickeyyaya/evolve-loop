//go:build integration

package ship

import (
	"context"
	"io"
	"path/filepath"
	"testing"
)

// shipDirect must hold the integrator lock across its add -A -> commit ->
// push critical section and release it exactly once.
func TestShipDirect_AcquiresAndReleasesShipLock(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "manual.txt"), "manual change\n")
	addRemote(t, repo)

	var acquired, released int
	var lockedPath string
	opts := Options{
		Class:            ClassManual,
		CommitMessage:    "fix: manual change",
		BypassCommitGate: true,
		Env:              map[string]string{"EVOLVE_SHIP_AUTO_CONFIRM": "1"},
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
		t.Fatalf("shipDirect ship lock acquired=%d released=%d, want 1/1 — shipDirect's git-mutating section is unprotected by acquireShipLock", acquired, released)
	}
	if filepath.Base(lockedPath) != "ship.lock" {
		t.Errorf("locked %q, want a path ending in ship.lock", lockedPath)
	}
}

// A dry-run shipDirect mutates nothing, so it must acquire zero locks (keeps
// dry-run pure + never creates the lock file), matching the existing
// shipFromWorktree convention. Calls shipDirect directly (bypassing Run()'s
// manual-class TTY gate).
func TestShipDirect_DryRun_SkipsShipLock(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "manual.txt"), "manual change\n")
	runGit(t, repo, "add", "manual.txt")

	var acquired int
	res := &RunResult{}
	opts := &Options{
		Class:         ClassManual,
		CommitMessage: "fix: manual change",
		ProjectRoot:   repo,
		DryRun:        true,
		Runner:        execRunner,
		Stdout:        io.Discard,
		Stderr:        io.Discard,
		shipLock:      func(string) (func(), error) { acquired++; return func() {}, nil },
	}
	if err := shipDirect(context.Background(), opts, res, "main"); err != nil {
		t.Fatalf("dry-run shipDirect errored: %v", err)
	}
	if acquired != 0 {
		t.Errorf("dry-run shipDirect must not acquire the ship lock; acquired=%d", acquired)
	}
}

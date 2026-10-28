//go:build integration

package ship

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestShipDirect_DryRun_LogsAndNoCommit(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "change.txt"), "dry-run change\n")
	runGit(t, repo, "add", "change.txt")

	headBefore := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))

	res := &RunResult{}
	opts := &Options{
		Class:         ClassManual,
		CommitMessage: "dry: should not commit",
		ProjectRoot:   repo,
		DryRun:        true,
		Runner:        execRunner,
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	err := shipDirect(context.Background(), opts, res, "main")
	if err != nil {
		t.Fatalf("DryRun shipDirect errored: %v", err)
	}

	if !containsLog(*res, "[DRY-RUN] would commit + push") {
		t.Errorf("missing DRY-RUN log; got: %v", res.Logs)
	}

	headAfter := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	if headBefore != headAfter {
		t.Errorf("DryRun should not commit; HEAD moved from %s to %s", headBefore, headAfter)
	}
}

func TestShipDirect_DryRun_NoStagedChanges_CleanExit(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	res := &RunResult{}
	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "dry: nothing staged",
		ProjectRoot:   repo,
		DryRun:        true,
		Runner:        execRunner,
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	if err := shipDirect(context.Background(), opts, res, "main"); err != nil {
		t.Fatalf("DryRun clean tree should not error: %v", err)
	}
	if !containsLog(*res, "no staged changes to ship") {
		t.Errorf("missing clean-exit log: %v", res.Logs)
	}
}

func TestRun_DryRun_ClassManual_WritesJournal_NoCommit(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "manual.txt"), "manual dry change\n")

	headBefore := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))

	res, err := runShip(t, repo, Options{
		Class:         ClassManual,
		CommitMessage: "manual: dry run",
		DryRun:        true,
		AutoConfirm:   "test",
	})
	if err != nil {
		t.Fatalf("DryRun manual errored: %v", err)
	}
	if res.ExitCode != ExitOK {
		t.Fatalf("DryRun manual want ExitOK, got %d", res.ExitCode)
	}
	headAfter := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	if headBefore != headAfter {
		t.Errorf("DryRun must not commit; HEAD moved %s → %s", headBefore, headAfter)
	}
}

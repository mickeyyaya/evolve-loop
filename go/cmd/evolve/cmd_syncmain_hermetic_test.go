package main

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestSyncMainFixture_TheWorkRepoDisablesGitMaintenance(t *testing.T) {
	t.Parallel()
	repo, _ := smInitRepoWithRemote(t)
	for _, pair := range gittest.MaintenanceConfig() {
		key, want := pair[0], pair[1]
		got, err := smGitAllowFail(t, repo, "config", "--get", key)
		if err != nil || strings.TrimSpace(got) != want {
			t.Errorf("%s of the work repo = %q (err %v), want %q: the sync runs git commit, fetch and merge here, and each can start a maintenance child that writes into .git while t.TempDir removes it", key, strings.TrimSpace(got), err, want)
		}
	}
}

func TestSyncMainFixture_ACloneMadeThroughTheHelperDisablesGitMaintenance(t *testing.T) {
	t.Parallel()
	_, bare := smInitRepoWithRemote(t)
	console := t.TempDir()
	smGit(t, t.TempDir(), "clone", "-q", bare, console)
	for _, pair := range gittest.MaintenanceConfig() {
		key, want := pair[0], pair[1]
		got, err := smGitAllowFail(t, console, "config", "--get", key)
		if err != nil || strings.TrimSpace(got) != want {
			t.Errorf("%s of a clone made through smGit = %q (err %v), want %q: the console clone commits and pushes, so it needs the same opt-out", key, strings.TrimSpace(got), err, want)
		}
	}
}

func TestSyncMainFixture_TheHelperStillReportsAFailingGitCommand(t *testing.T) {
	t.Parallel()
	repo, _ := smInitRepoWithRemote(t)
	if out, err := smGitAllowFail(t, repo, "rev-parse", "--verify", "refs/heads/no-such-branch"); err == nil {
		t.Errorf("smGitAllowFail on a missing ref = nil error (output %q), want the git failure reported", out)
	}
	if got := strings.TrimSpace(smGit(t, repo, "rev-parse", "--abbrev-ref", "HEAD")); got != "main" {
		t.Errorf("smGit rev-parse --abbrev-ref HEAD = %q, want main: the helper must pass its arguments through unchanged", got)
	}
}

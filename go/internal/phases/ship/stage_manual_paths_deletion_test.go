//go:build integration

package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func manualPathsRepo(t *testing.T) string {
	t.Helper()
	repo := makeRepo(t)
	excludeCommitGate(t, repo)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "doomed.txt"), "to be deleted\n")
	runGit(t, repo, "add", "doomed.txt")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "seed doomed")
	return repo
}

func shipManualPaths(t *testing.T, repo string, named ...string) (RunResult, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(WithPaths(context.Background(), named), 30*time.Second)
	defer cancel()
	return Run(ctx, Options{
		Class:            ClassManual,
		CommitMessage:    "chore: drop doomed",
		AutoConfirm:      "test",
		BypassCommitGate: true,
		ProjectRoot:      repo,
		PluginRoot:       repo,
		ShipBinaryPath:   filepath.Join(repo, "ship-binary-fixture"),
		Runner:           execRunner,
		Stdin:            strings.NewReader(""),
		Stdout:           io.Discard,
		Stderr:           io.Discard,
	})
}

func TestManualShipPaths_AlreadyStagedDeletionShipsAndExitsZero(t *testing.T) {
	t.Parallel()
	repo := manualPathsRepo(t)
	if err := os.Remove(filepath.Join(repo, "doomed.txt")); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "-A", "--", "doomed.txt")

	res, err := shipManualPaths(t, repo, "doomed.txt")

	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("a staged deletion named by --paths must ship: exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	shipped := runGitOut(t, repo, "show", "--name-status", "--pretty=format:", "HEAD")
	if strings.TrimSpace(shipped) != "D\tdoomed.txt" {
		t.Errorf("the ship commit must carry exactly the deletion, got %q", shipped)
	}
}

func TestManualShipPaths_StagedDeletionAndModifiedPathShipTogether(t *testing.T) {
	t.Parallel()
	repo := manualPathsRepo(t)
	if err := os.Remove(filepath.Join(repo, "doomed.txt")); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "-A", "--", "doomed.txt")
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nedited\n")
	mustWrite(t, filepath.Join(repo, "sibling-leak.txt"), "undeclared\n")

	res, err := shipManualPaths(t, repo, "doomed.txt", "fixture.txt")

	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("mixed --paths must ship: exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	shipped := runGitOut(t, repo, "show", "--name-status", "--pretty=format:", "HEAD")
	for _, want := range []string{"D\tdoomed.txt", "M\tfixture.txt"} {
		if !strings.Contains(shipped, want) {
			t.Errorf("ship commit lacks %q: %q", want, shipped)
		}
	}
	if strings.Contains(shipped, "sibling-leak.txt") {
		t.Errorf("an undeclared path rode into the commit: %q", shipped)
	}
}

func TestManualShipPaths_PathKnownNowhereRefusesNamingThePath(t *testing.T) {
	t.Parallel()
	repo := manualPathsRepo(t)
	if err := os.Remove(filepath.Join(repo, "doomed.txt")); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "-A", "--", "doomed.txt")
	before := headSHA(t, repo)

	res, err := shipManualPaths(t, repo, "doomed.txt", "ghost-never-existed.txt")

	if res.ExitCode == ExitOK {
		t.Fatalf("a path in no index, no HEAD and no disk must refuse the ship: logs=%v", res.Logs)
	}
	se := mustShipErr(t, err)
	if se.Code != core.CodeGitIO && se.Code != core.CodeGitStageFailed {
		t.Errorf("unexpected error code %s", se.Code)
	}
	if !strings.Contains(se.Message, "ghost-never-existed.txt") {
		t.Errorf("the refusal must name the unknown path, got message %q", se.Message)
	}
	if strings.Contains(se.Message, "doomed.txt") {
		t.Errorf("the refusal must not blame the valid staged deletion, got message %q", se.Message)
	}
	if after := headSHA(t, repo); after != before {
		t.Errorf("a refused ship moved HEAD: %s -> %s", before, after)
	}
}

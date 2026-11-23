//go:build acs

package cycle2624

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func repoWithTrackedDoomedFile(t *testing.T) string {
	t.Helper()
	repo := shiptest.MakeRepo(t)
	shiptest.AddRemote(t, repo)
	shiptest.MustWrite(t, filepath.Join(repo, ".git", "info", "exclude"), ".commit-gate/\n")
	shiptest.MustWrite(t, filepath.Join(repo, "doomed.txt"), "to be deleted\n")
	shiptest.RunGit(t, repo, "add", "doomed.txt")
	shiptest.RunGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "seed doomed")
	return repo
}

func shipManualPaths(t *testing.T, repo string, named ...string) (ship.RunResult, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(ship.WithPaths(context.Background(), named), 60*time.Second)
	defer cancel()
	return ship.Run(ctx, ship.Options{
		Class:            ship.ClassManual,
		CommitMessage:    "chore: drop doomed",
		AutoConfirm:      "test",
		BypassCommitGate: true,
		ProjectRoot:      repo,
		PluginRoot:       repo,
		ShipBinaryPath:   filepath.Join(repo, "ship-binary-fixture"),
		Stdin:            strings.NewReader(""),
		Stdout:           io.Discard,
		Stderr:           io.Discard,
	})
}

func headChanges(t *testing.T, repo string) string {
	t.Helper()
	return shiptest.RunGitOut(t, repo, "show", "--name-status", "--pretty=format:", "HEAD")
}

func TestC2624_001_PathsShipsAnAlreadyStagedDeletionAndExitsZero(t *testing.T) {
	repo := repoWithTrackedDoomedFile(t)
	if err := os.Remove(filepath.Join(repo, "doomed.txt")); err != nil {
		t.Fatal(err)
	}
	shiptest.RunGit(t, repo, "add", "-A", "--", "doomed.txt")

	res, err := shipManualPaths(t, repo, "doomed.txt")

	if err != nil || res.ExitCode != ship.ExitOK {
		t.Fatalf("RED: a staged deletion named by --paths must ship, exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if got := strings.TrimSpace(headChanges(t, repo)); got != "D\tdoomed.txt" {
		t.Errorf("RED: the ship commit must carry exactly the deletion, got %q", got)
	}
}

func TestC2624_002_PathKnownNowhereStillRefusesAndNamesThePath(t *testing.T) {
	repo := repoWithTrackedDoomedFile(t)
	if err := os.Remove(filepath.Join(repo, "doomed.txt")); err != nil {
		t.Fatal(err)
	}
	shiptest.RunGit(t, repo, "add", "-A", "--", "doomed.txt")
	before := shiptest.HeadSHA(t, repo)

	res, err := shipManualPaths(t, repo, "doomed.txt", "ghost-never-existed.txt")

	if res.ExitCode == ship.ExitOK {
		t.Fatalf("RED: a path in no index, no HEAD and no disk must refuse the ship, logs=%v", res.Logs)
	}
	se, ok := core.AsShipError(err)
	if !ok {
		t.Fatalf("RED: want a *core.ShipError, got %T: %v", err, err)
	}
	if !strings.Contains(se.Message, "ghost-never-existed.txt") {
		t.Errorf("RED: the refusal must name the unknown path, got %q", se.Message)
	}
	if strings.Contains(se.Message, "doomed.txt") {
		t.Errorf("RED: the refusal must not blame the valid staged deletion, got %q", se.Message)
	}
	if after := shiptest.HeadSHA(t, repo); after != before {
		t.Errorf("a refused ship moved HEAD: %s -> %s", before, after)
	}
}

func TestC2624_003_TheStagedDeletionShipTestExistsAndPasses(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	const name = "TestManualShipPaths_AlreadyStagedDeletionShipsAndExitsZero"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-tags", "integration", "-count=1", "-run", "^"+name+"$", "-v", "./internal/phases/ship")
	cmd.Dir = goDir
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("RED: go test failed: %v\n%s", err, acsassert.OutputTail(string(out), 30))
	}
	if !strings.Contains(string(out), "--- PASS: "+name) {
		t.Errorf("RED: %s did not run and pass:\n%s", name, acsassert.OutputTail(string(out), 30))
	}
}

func TestC2624_004_PathsStillShipsAnUnstagedDeletion(t *testing.T) {
	repo := repoWithTrackedDoomedFile(t)
	if err := os.Remove(filepath.Join(repo, "doomed.txt")); err != nil {
		t.Fatal(err)
	}

	res, err := shipManualPaths(t, repo, "doomed.txt")

	if err != nil || res.ExitCode != ship.ExitOK {
		t.Fatalf("an unstaged deletion named by --paths must keep shipping, exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if got := strings.TrimSpace(headChanges(t, repo)); got != "D\tdoomed.txt" {
		t.Errorf("the ship commit must carry exactly the deletion, got %q", got)
	}
}

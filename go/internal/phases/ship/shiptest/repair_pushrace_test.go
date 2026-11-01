//go:build integration

package shiptest_test

import (
	"context"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func failFirstPushRunner() ship.CmdRunner {
	failed := false
	return func(ctx context.Context, name, cwd string, args, env []string,
		stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && !failed && slices.Contains(args, "push") {
			failed = true
			return 1, nil
		}
		return sysexec.DefaultRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
}

func TestRepair_PushRace_FetchFFRetry_Succeeds(t *testing.T) {
	t.Parallel()
	repo := shiptest.MakeRepo(t)
	shiptest.AddRemote(t, repo)
	shiptest.RunGit(t, repo, "push", "-q", "origin", "main")
	shiptest.MustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\naudited edit\n")
	shiptest.SeedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, ship.Options{
		Class:         ship.ClassCycle,
		CommitMessage: "feat: push race retry",
		Runner:        failFirstPushRunner(),
	})
	if res.ExitCode != ship.ExitOK {
		t.Fatalf("transient push rejection must ff-retry; got exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if res.RepairAttempted != string(core.CodeGitPushRejected) {
		t.Errorf("RepairAttempted = %q, want %s", res.RepairAttempted, core.CodeGitPushRejected)
	}
	if got, head := shiptest.RemoteHeadSHA(t, repo), shiptest.HeadSHA(t, repo); got != head {
		t.Errorf("remote main = %s, want pushed HEAD %s", got, head)
	}
}

func TestRepair_PushRace_Diverged_ReclassifiedNeedsReaudit(t *testing.T) {
	t.Parallel()
	repo := shiptest.MakeRepo(t)
	bare := shiptest.AddRemote(t, repo)
	shiptest.RunGit(t, repo, "push", "-q", "origin", "main")
	shiptest.MustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\naudited edit\n")
	shiptest.SeedAudit(t, repo, "PASS")

	shiptest.PushDivergentCommit(t, bare)
	divergedRemote := shiptest.RemoteHeadSHA(t, repo)

	res, err := runShip(t, repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "feat: diverged push"})
	if res.ExitCode == ship.ExitOK {
		t.Fatalf("diverged push must NOT self-heal; got ExitOK (logs=%v)", res.Logs)
	}
	se := shiptest.WantShipErr(t, err, core.CodeGitPushRejected, core.ShipClassPrecondition, "")
	if se.Debug["repair_outcome"] != "needs-reaudit" {
		t.Errorf("Debug[repair_outcome] = %q, want needs-reaudit", se.Debug["repair_outcome"])
	}
	mainFiles := shiptest.RunGitOut(t, repo, "log", "-1", "--name-only", "--format=")
	if !strings.Contains(mainFiles, "fixture.txt") {
		t.Errorf("local commit lost; HEAD files: %q", mainFiles)
	}
	if got := shiptest.RemoteHeadSHA(t, repo); got != divergedRemote {
		t.Errorf("remote main moved (%s → %s) — must never force-push", divergedRemote, got)
	}
}

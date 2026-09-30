//go:build integration

package ship

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// failFirstPushRunner wraps execRunner, failing the first `git push` with
// exit 1 (simulated transient rejection) and delegating everything else.
func failFirstPushRunner() CmdRunner {
	failed := false
	return func(ctx context.Context, name, cwd string, args, env []string,
		stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && !failed && hasArg(args, "push") {
			failed = true
			return 1, nil
		}
		return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
}

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestRepair_PushRace_FetchFFRetry_Succeeds(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	runGit(t, repo, "push", "-q", "origin", "main")
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\naudited edit\n")
	seedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, Options{
		Class:         ClassCycle,
		CommitMessage: "feat: push race retry",
		Runner:        failFirstPushRunner(),
	})
	if res.ExitCode != ExitOK {
		t.Fatalf("transient push rejection must ff-retry; got exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if res.RepairAttempted != string(core.CodeGitPushRejected) {
		t.Errorf("RepairAttempted = %q, want %s", res.RepairAttempted, core.CodeGitPushRejected)
	}
	if got, head := remoteHeadSHA(t, repo), headSHA(t, repo); got != head {
		t.Errorf("remote main = %s, want pushed HEAD %s", got, head)
	}
}

func TestRepair_PushRace_Diverged_ReclassifiedNeedsReaudit(t *testing.T) {
	repo := makeRepo(t)
	bare := addRemote(t, repo)
	runGit(t, repo, "push", "-q", "origin", "main")
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\naudited edit\n")
	seedAudit(t, repo, "PASS")

	pushDivergentCommit(t, bare)
	divergedRemote := remoteHeadSHA(t, repo)

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: diverged push"})
	if res.ExitCode == ExitOK {
		t.Fatalf("diverged push must NOT self-heal; got ExitOK (logs=%v)", res.Logs)
	}
	se := wantShipErr(t, err, core.CodeGitPushRejected, core.ShipClassPrecondition, "")
	if se.Debug["repair_outcome"] != "needs-reaudit" {
		t.Errorf("Debug[repair_outcome] = %q, want needs-reaudit", se.Debug["repair_outcome"])
	}
	mainFiles := runGitOut(t, repo, "log", "-1", "--name-only", "--format=")
	if !strings.Contains(mainFiles, "fixture.txt") {
		t.Errorf("local commit lost; HEAD files: %q", mainFiles)
	}
	if got := remoteHeadSHA(t, repo); got != divergedRemote {
		t.Errorf("remote main moved (%s → %s) — must never force-push", divergedRemote, got)
	}
}

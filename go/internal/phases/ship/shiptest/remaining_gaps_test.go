//go:build integration

package shiptest_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func TestRun_PostShipError_LogsWarnAndContinues(t *testing.T) {
	t.Parallel()
	repo := shiptest.MakeRepo(t)
	shiptest.AddRemote(t, repo)
	shiptest.MustWrite(t, filepath.Join(repo, "warn.txt"), "change\n")
	shiptest.SeedAudit(t, repo, "PASS")

	csPath := filepath.Join(repo, ".evolve", "cycle-state.json")
	_ = os.Remove(csPath)
	if err := os.MkdirAll(csPath, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(csPath) })

	var stderrBuf strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := ship.Run(ctx, ship.Options{
		Class:         ship.ClassCycle,
		CommitMessage: "feat: post-ship warn",
		ProjectRoot:   repo,
		WorkspacePath: filepath.Join(repo, ".evolve", "runs", "cycle-1"),
		CycleID:       1,
		RunID:         "test-run", AuditRound: 1,
		PluginRoot:     repo,
		ShipBinaryPath: filepath.Join(repo, "ship-binary-fixture"),
		Runner:         sysexec.DefaultRunner,
		Stdin:          strings.NewReader(""),
		Stdout:         io.Discard,
		Stderr:         &stderrBuf,
	})
	if err != nil {
		t.Fatalf("postShip WARN must not propagate as error, got: %v", err)
	}
	if res.ExitCode != ship.ExitOK {
		t.Fatalf("want ExitOK despite postShip error, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	found := false
	for _, l := range res.Logs {
		if strings.Contains(l, "post-ship hook error") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("want 'post-ship hook error' WARN in logs; got %v", res.Logs)
	}
}

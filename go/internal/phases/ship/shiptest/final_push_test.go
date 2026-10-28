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

func TestRun_NilPluginRoot_DefaultsToProjectRoot(t *testing.T) {
	t.Parallel()
	repo := shiptest.MakeRepo(t)
	shiptest.AddRemote(t, repo)
	shiptest.MustWrite(t, filepath.Join(repo, "p.txt"), "change\n")
	shiptest.SeedAudit(t, repo, "PASS")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := ship.Run(ctx, ship.Options{
		Class:          ship.ClassCycle,
		CommitMessage:  "feat: nil pluginroot",
		ProjectRoot:    repo,
		PluginRoot:     "",
		ShipBinaryPath: filepath.Join(repo, "ship-binary-fixture"),
		Runner:         sysexec.DefaultRunner,
		Stdin:          strings.NewReader(""),
		Stdout:         io.Discard,
		Stderr:         io.Discard,
	})
	if err != nil {
		t.Fatalf("nil PluginRoot ship errored: %v (logs=%v)", err, res.Logs)
	}
	if res.ExitCode != ship.ExitOK {
		t.Fatalf("want ExitOK, got %d", res.ExitCode)
	}
}

func TestRun_NilRunner_DefaultsToExecRunner(t *testing.T) {
	t.Parallel()
	repo := shiptest.MakeRepo(t)
	shiptest.AddRemote(t, repo)
	shiptest.MustWrite(t, filepath.Join(repo, "q.txt"), "change\n")
	shiptest.SeedAudit(t, repo, "PASS")

	ctx := context.Background()
	opts := ship.Options{
		Class:          ship.ClassCycle,
		CommitMessage:  "feat: nil runner",
		ProjectRoot:    repo,
		PluginRoot:     repo,
		ShipBinaryPath: filepath.Join(repo, "ship-binary-fixture"),
		Runner:         nil,
		Stdin:          strings.NewReader(""),
		Stdout:         io.Discard,
		Stderr:         io.Discard,
	}
	res, err := ship.Run(ctx, opts)
	if err != nil {
		t.Fatalf("nil Runner ship errored: %v (logs=%v)", err, res.Logs)
	}
	if res.ExitCode != ship.ExitOK {
		t.Fatalf("want ExitOK, got %d", res.ExitCode)
	}
}

func TestShipFromWorktree_EmptyCycleBranch_Errors(t *testing.T) {
	t.Parallel()
	repo := shiptest.MakeRepo(t)
	shiptest.AddRemote(t, repo)
	wt := shiptest.MakeWorktree(t, repo, "real-cycle-branch")
	shiptest.MustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":100,"active_worktree":"`+wt+`"}`)
	shiptest.SeedAudit(t, repo, "PASS")

	base := sysexec.DefaultRunner
	hijack := func(ctx context.Context, name, cwd string, args, env []string,
		stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && len(args) >= 4 && args[0] == "-C" && args[1] == wt {
			for _, a := range args {
				if a == "symbolic-ref" {
					return 0, nil
				}
			}
		}
		return base(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}

	res, err := runShip(t, repo, ship.Options{
		Class:         ship.ClassCycle,
		CommitMessage: "feat: empty branch",
		Runner:        hijack,
	})
	_ = res
	if err == nil || !strings.Contains(err.Error(), "empty cycle branch") {
		t.Fatalf("empty cycleBranch must error; got err=%v logs=%v", err, res.Logs)
	}
}

func TestShipFromWorktree_WriteShipBindingUsesFrozenCycleIdentity(t *testing.T) {
	t.Parallel()
	repo := shiptest.MakeRepo(t)
	shiptest.AddRemote(t, repo)
	wt := shiptest.MakeWorktree(t, repo, "binding-warn-branch")
	shiptest.MustWrite(t, filepath.Join(wt, "warn-file.txt"), "content\n")
	csPath := filepath.Join(repo, ".evolve", "cycle-state.json")
	shiptest.MustWrite(t, csPath, `{"cycle_id":99,"active_worktree":"`+wt+`"}`)
	shiptest.SeedAudit(t, repo, "PASS")

	base := sysexec.DefaultRunner
	hijack := func(ctx context.Context, name, cwd string, args, env []string,
		stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		rc, err := base(ctx, name, cwd, args, env, stdin, stdout, stderr)
		if name == "git" && len(args) > 0 && args[0] == "push" && rc == 0 {
			_ = os.WriteFile(csPath, []byte(`{"phase":"done"}`), 0o644)
		}
		return rc, err
	}

	res, err := runShip(t, repo, ship.Options{
		Class:         ship.ClassCycle,
		CommitMessage: "feat: warn binding",
		Runner:        hijack,
	})
	if err != nil {
		t.Fatalf("writeShipBinding warn must not fail ship: %v", err)
	}
	if res.ExitCode != ship.ExitOK {
		t.Fatalf("want ExitOK, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if containsLog(res, "WARN: could not write ship-binding.json") {
		t.Errorf("mutable cycle-state rewrite redirected the frozen binding: %v", res.Logs)
	}
	if _, statErr := os.Stat(filepath.Join(repo, ".evolve", "runs", "cycle-99", "ship-binding.json")); statErr != nil {
		t.Fatalf("typed cycle-99 ship binding missing: %v", statErr)
	}
}

// error_paths_test.go — fault-injected coverage for the git-operation
// error branches that the real-git happy-path matrix can't reach, plus the
// finalize() exit-code classifier. Uses faultRunner to fail exactly one git
// subcommand while delegating the rest to real git, so the code under test
// runs against a genuine repo right up to the injected failure.
package ship

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// faultRunner returns (exit, failErr) for the git subcommand matching
// failKey (e.g. "git commit") and delegates every other call to execRunner.
func faultRunner(failKey string, exit int, failErr error) CmdRunner {
	return func(ctx context.Context, name string, args, env []string, cwd string,
		stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		key := name
		for i := 0; i < len(args); i++ {
			a := args[i]
			if !strings.HasPrefix(a, "-") {
				key = name + " " + a
				break
			}
			if a == "-C" || a == "-c" {
				i++
			}
		}
		if key == failKey {
			return exit, failErr
		}
		return execRunner(ctx, name, args, env, cwd, stdin, stdout, stderr)
	}
}

func TestShipDirect_GitAddFails_Errors(t *testing.T) {
	repo := makeRepo(t)
	opts := &Options{Class: ClassManual, CommitMessage: "msg", ProjectRoot: repo,
		Runner: faultRunner("git add", 1, nil), Stdout: io.Discard, Stderr: io.Discard}
	err := shipDirect(context.Background(), opts, &RunResult{}, "main")
	if err == nil || !strings.Contains(err.Error(), "git add -A failed") {
		t.Fatalf("want git-add-failed error, got %v", err)
	}
}

func TestShipDirect_GitCommitFails_Errors(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "change.txt"), "staged change\n")
	opts := &Options{Class: ClassCycle, CommitMessage: "msg", ProjectRoot: repo,
		Runner: faultRunner("git commit", 1, nil), Stdout: io.Discard, Stderr: io.Discard}
	err := shipDirect(context.Background(), opts, &RunResult{}, "main")
	if err == nil || !strings.Contains(err.Error(), "git commit failed") {
		t.Fatalf("want commit-failed error, got %v", err)
	}
}

func TestShipDirect_GitPushFails_Errors(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "change.txt"), "staged change\n")
	opts := &Options{Class: ClassCycle, CommitMessage: "msg", ProjectRoot: repo,
		Runner: faultRunner("git push", 1, nil), Stdout: io.Discard, Stderr: io.Discard}
	err := shipDirect(context.Background(), opts, &RunResult{}, "main")
	if err == nil || !strings.Contains(err.Error(), "git push failed") {
		t.Fatalf("want push-failed error, got %v", err)
	}
}

func TestShipDirect_NoStagedChanges_CleanExit(t *testing.T) {
	repo := makeRepo(t) // clean tree, nothing to stage
	opts := &Options{Class: ClassCycle, CommitMessage: "msg", ProjectRoot: repo,
		Runner: execRunner, Stdout: io.Discard, Stderr: io.Discard}
	res := &RunResult{}
	if err := shipDirect(context.Background(), opts, res, "main"); err != nil {
		t.Fatalf("clean tree must exit cleanly, got %v", err)
	}
	if !containsLog(*res, "no staged changes to ship") {
		t.Errorf("missing clean-exit log: %v", res.Logs)
	}
}

func TestAtomicShip_DetachedHEAD_Refuses(t *testing.T) {
	repo := makeRepo(t)
	opts := &Options{Class: ClassCycle, CommitMessage: "msg", ProjectRoot: repo,
		Runner: faultRunner("git symbolic-ref", 1, nil), Stdout: io.Discard, Stderr: io.Discard}
	err := atomicShip(context.Background(), opts, &RunResult{})
	if err == nil || !strings.Contains(err.Error(), "detached HEAD") {
		t.Fatalf("want detached-HEAD refusal, got %v", err)
	}
}

func TestComputeTreeStateSHA_GitExitGreaterThan1_Errors(t *testing.T) {
	repo := makeRepo(t)
	opts := &Options{ProjectRoot: repo, Runner: faultRunner("git diff", 2, nil)}
	if _, err := computeTreeStateSHA(context.Background(), opts); err == nil {
		t.Fatal("git diff exit>1 must error")
	}
}

func TestComputeTreeStateSHA_RunnerError_Errors(t *testing.T) {
	repo := makeRepo(t)
	opts := &Options{ProjectRoot: repo, Runner: faultRunner("git diff", -1, errors.New("boom"))}
	if _, err := computeTreeStateSHA(context.Background(), opts); err == nil {
		t.Fatal("runner error must propagate")
	}
}

func TestFinalize_ClassifiesExitCodes(t *testing.T) {
	opts := &Options{ProjectRoot: t.TempDir()} // DryRun=false → journal no-op
	t.Run("integrity-error", func(t *testing.T) {
		// An integrity-class ShipError maps to ExitIntegrity; finalize now keys
		// off the structured Class, not the Go type.
		breach := shipErr(core.CodeIntegrityTreeDrift, core.ShipClassIntegrity, core.StagePostShip, "breach")
		out, err := finalize(context.Background(), opts, &RunResult{}, breach, "r")
		if err == nil || out.ExitCode != ExitIntegrity {
			t.Errorf("want ExitIntegrity + err, got code=%d err=%v", out.ExitCode, err)
		}
	})
	t.Run("generic-error", func(t *testing.T) {
		out, err := finalize(context.Background(), opts, &RunResult{}, errors.New("boom"), "r")
		if err == nil || out.ExitCode != ExitFailure {
			t.Errorf("want ExitFailure + err, got code=%d err=%v", out.ExitCode, err)
		}
	})
	t.Run("nil-error", func(t *testing.T) {
		out, err := finalize(context.Background(), opts, &RunResult{}, nil, "r")
		if err != nil || out.ExitCode != ExitOK {
			t.Errorf("want ExitOK + nil, got code=%d err=%v", out.ExitCode, err)
		}
	})
}

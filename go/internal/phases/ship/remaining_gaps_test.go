//go:build integration

package ship

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAtomicShip_CurrentBranchRunnerError_Errors(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: branch fail",
		ProjectRoot:   repo,
		Runner:        faultRunner("git symbolic-ref", -1, errors.New("symbolic-ref exploded")),
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	err := atomicShip(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("want currentBranch error, got nil")
	}
}

func TestShipDirect_BuildDiffFooterNameStatusError_Errors(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "change.txt"), "staged change\n")

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: footer fail",
		ProjectRoot:   repo,
		Runner: func(ctx context.Context, name, cwd string, args, env []string,
			stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			if name == "git" && argsContain(args, "--name-status") {
				return -1, errors.New("name-status exploded")
			}
			return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
		},
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	err := shipDirect(context.Background(), opts, &RunResult{}, "main")
	if err == nil || !strings.Contains(err.Error(), "name-status") {
		t.Fatalf("want name-status error, got %v", err)
	}
}

func TestPromoteInbox_CycleStateReadError_ReturnsError(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)

	csPath := filepath.Join(repo, ".evolve", "cycle-state.json")
	if err := os.MkdirAll(csPath, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(csPath) })

	opts := &Options{ProjectRoot: repo}
	err := promoteInbox(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("want readStateMap error, got nil")
	}
}

func TestVerifyManualConfirm_GitAddAFails_Errors(t *testing.T) {
	t.Parallel()
	opts := &Options{
		ProjectRoot: t.TempDir(),
		Runner: func(ctx context.Context, name, cwd string, args, env []string,
			stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			if name == "git" && len(args) > 0 && args[0] == "add" {
				return -1, errors.New("add -A exploded")
			}
			return 0, nil
		},
		Stderr: io.Discard,
		Stdin:  strings.NewReader(""),
	}
	err := verifyManualConfirm(context.Background(), opts, &RunResult{})
	if err == nil || !strings.Contains(err.Error(), "git add -A failed") {
		t.Fatalf("want 'git add -A failed' error, got %v", err)
	}
}

func TestVerifyTrivial_DiffNameOnlyRunnerError_Errors(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)

	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"cycle_size_estimate":"trivial"}`)

	opts := &Options{
		ProjectRoot: repo,
		Runner: func(ctx context.Context, name, cwd string, args, env []string,
			stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			if name == "git" && argsContain(args, "--name-only") && !argsContain(args, "--cached") {
				return -1, errors.New("diff name-only exploded")
			}
			return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
		},
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	err := verifyTrivial(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("want runner error from diff --name-only, got nil")
	}
}

func TestVerifyTrivial_LSFilesRunnerError_Errors(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)

	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"cycle_size_estimate":"trivial"}`)

	opts := &Options{
		ProjectRoot: repo,
		Runner: func(ctx context.Context, name, cwd string, args, env []string,
			stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			if name == "git" && argsContain(args, "ls-files") {
				return -1, errors.New("ls-files exploded")
			}
			return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
		},
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	err := verifyTrivial(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("want runner error from ls-files, got nil")
	}
}

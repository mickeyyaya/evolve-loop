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

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestVerifySelfSHA_UnreadableBinary_Errors(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)

	bin := filepath.Join(repo, "unreadable-bin")
	mustWrite(t, bin, "binary content\n")
	if err := os.Chmod(bin, 0o000); err != nil {
		t.Skip("cannot chmod 0000 on this system")
	}
	t.Cleanup(func() { _ = os.Chmod(bin, 0o644) })

	opts := &Options{
		ProjectRoot:    repo,
		PluginRoot:     repo,
		ShipBinaryPath: bin,
		Runner:         execRunner,
		Stdin:          strings.NewReader(""),
		Stdout:         io.Discard,
		Stderr:         io.Discard,
	}
	err := verifySelfSHA(context.Background(), opts, &RunResult{})
	if err == nil || !strings.Contains(err.Error(), "cannot SHA ship binary") {
		t.Fatalf("want 'cannot SHA ship binary' error, got %v", err)
	}
}

func TestVerifySelfSHA_RepinWriteStateFails_Errors(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	bin := filepath.Join(repo, "ship-binary-fixture")
	preSeedTOFU(t, repo, bin)

	mustWrite(t, filepath.Join(repo, ".evolve", "state.json"), "{}\n")

	evolveDir := filepath.Join(repo, ".evolve")
	if err := os.Chmod(evolveDir, 0o555); err != nil {
		t.Skip("cannot chmod .evolve dir")
	}
	t.Cleanup(func() { _ = os.Chmod(evolveDir, 0o755) })

	opts := &Options{
		ProjectRoot:    repo,
		PluginRoot:     repo,
		ShipBinaryPath: bin,
		Runner:         execRunner,
		Stdin:          strings.NewReader(""),
		Stdout:         io.Discard,
		Stderr:         io.Discard,
	}
	err := verifySelfSHA(context.Background(), opts, &RunResult{})
	var se *core.ShipError
	if err == nil || !errors.As(err, &se) || se.Code != core.CodeStateIO {
		t.Fatalf("want a STATE_IO refusal on read-only .evolve, got %v", err)
	}
}

func TestVerifyManualConfirm_DiffCachedQuietRunnerError_Errors(t *testing.T) {
	t.Parallel()
	call := 0
	opts := &Options{
		ProjectRoot: t.TempDir(),
		Runner: func(ctx context.Context, name, cwd string, args, env []string,
			stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			if name != "git" {
				return 0, nil
			}
			call++
			if call == 1 {
				return 0, nil
			}
			return -1, errors.New("diff quiet runner error")
		},
		Stderr: io.Discard,
		Stdin:  strings.NewReader(""),
	}
	err := verifyManualConfirm(context.Background(), opts, &RunResult{})
	if err == nil || !strings.Contains(err.Error(), "diff --cached --quiet failed") {
		t.Fatalf("want 'diff --cached --quiet failed' error, got %v", err)
	}
}

func TestVerifyTrivial_StagedNameOnlyRunnerError_Errors(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"cycle_size_estimate":"trivial"}`)

	opts := &Options{
		ProjectRoot: repo,
		Runner: func(ctx context.Context, name, cwd string, args, env []string,
			stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			if name == "git" && argsContain(args, "--cached") && argsContain(args, "--name-only") {
				return -1, errors.New("staged name-only exploded")
			}
			return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
		},
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	err := verifyTrivial(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("want runner error from diff --cached --name-only, got nil")
	}
}

func TestVerifyAuditBinding_UnreadableArtifact_SHA256Error(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")

	auditPath := filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")
	if err := os.Chmod(auditPath, 0o000); err != nil {
		t.Skip("cannot chmod 0000 on this system")
	}
	t.Cleanup(func() { _ = os.Chmod(auditPath, 0o644) })

	opts := auditOpts(t, repo)
	err := verifyAuditBinding(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("want sha256File error on unreadable artifact, got nil")
	}
	if _, ok := err.(*IntegrityError); ok {
		t.Errorf("sha256 read error should be plain error, not IntegrityError; got %v", err)
	}
}

func TestVerifyAuditBinding_RevParseHeadRunnerError_Errors(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")

	opts := auditOpts(t, repo)
	opts.Runner = func(ctx context.Context, name, cwd string, args, env []string,
		stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && argsContain(args, "rev-parse") && argsContain(args, "HEAD") &&
			!argsContain(args, "HEAD^{tree}") && !argsContain(args, "--abbrev-ref") {
			return -1, errors.New("rev-parse HEAD exploded")
		}
		return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
	err := verifyAuditBinding(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("want rev-parse HEAD error, got nil")
	}
	if _, ok := err.(*IntegrityError); ok {
		t.Errorf("runner error should be plain error, not IntegrityError; got %v", err)
	}
}

func TestVerifyAuditBinding_ComputeTreeSHARunnerError_Errors(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")

	opts := auditOpts(t, repo)
	opts.Runner = func(ctx context.Context, name, cwd string, args, env []string,
		stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && argsContain(args, "diff") && argsContain(args, "HEAD") {
			return -1, errors.New("git diff HEAD exploded")
		}
		return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
	err := verifyAuditBinding(context.Background(), opts, &RunResult{})
	if err == nil {
		t.Fatal("want computeTreeStateSHA error, got nil")
	}
}

func TestShipDirect_CommitPrefixGateRejects_Errors(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "staged.txt"), "staged change\n")

	manifest := `{"prefixes":{"docs":{"required_paths":["docs/"]}}}`
	mustWrite(t, filepath.Join(repo, ".evolve", "commit-prefix-scope.json"), manifest)

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "docs: touches the wrong paths",
		ProjectRoot:   repo,
		Runner:        execRunner,
		Stdin:         strings.NewReader(""),
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	err := shipDirect(context.Background(), opts, &RunResult{}, "main")
	if err == nil || !strings.Contains(err.Error(), "commit-prefix-gate") {
		t.Fatalf("want commit-prefix-gate rejection, got %v", err)
	}
}

func TestShipFromWorktree_CommitPrefixGateRejects_Errors(t *testing.T) {
	t.Parallel()
	repo, wt := makeWorktreeScenario(t)

	manifest := `{"prefixes":{"docs":{"required_paths":["docs/"]}}}`
	mustWrite(t, filepath.Join(wt, ".evolve", "commit-prefix-scope.json"), manifest)

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "docs: touches the wrong paths",
		ProjectRoot:   repo,
		Runner:        execRunner,
		Stdin:         strings.NewReader(""),
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt)
	if err == nil || !strings.Contains(err.Error(), "commit-prefix-gate") {
		t.Fatalf("want commit-prefix-gate rejection, got %v", err)
	}
}

func TestShipFromWorktree_GitCommitFails_Errors(t *testing.T) {
	t.Parallel()
	repo, wt := makeWorktreeScenario(t)

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: commit fail",
		ProjectRoot:   repo,
		Runner:        faultRunner("git commit-tree", 128, nil),
		Stdin:         strings.NewReader(""),
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt)
	if err == nil || !strings.Contains(err.Error(), "git commit-tree in worktree failed") {
		t.Fatalf("want 'git commit-tree in worktree failed' error, got %v", err)
	}
}

func TestShipFromWorktree_WriteTreeFails_Errors(t *testing.T) {
	t.Parallel()
	repo, wt := makeWorktreeScenario(t)

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: write-tree fail",
		ProjectRoot:   repo,
		Runner: func(ctx context.Context, name, cwd string, args, env []string,
			stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			if name == "git" && argsContain(args, "write-tree") {
				return -1, errors.New("write-tree exploded")
			}
			return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
		},
		internalAuditBoundTreeSHA: "someboundsha",
		Stdin:                     strings.NewReader(""),
		Stdout:                    io.Discard,
		Stderr:                    io.Discard,
	}
	err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt)
	if err == nil {
		t.Fatal("want write-tree error, got nil")
	}
}

func TestShipFromWorktree_WriteTreeEmptyOutput_FailsClosed(t *testing.T) {
	t.Parallel()
	repo, wt := makeWorktreeScenario(t)

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: write-tree empty",
		ProjectRoot:   repo,
		Runner: func(ctx context.Context, name, cwd string, args, env []string,
			stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			if name == "git" && argsContain(args, "write-tree") {
				return 0, nil
			}
			return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
		},
		internalAuditBoundTreeSHA: "someboundsha",
		Stdin:                     strings.NewReader(""),
		Stdout:                    io.Discard,
		Stderr:                    io.Discard,
	}
	err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt)
	if err == nil {
		t.Fatal("want fail-closed error on empty write-tree output, got nil")
	}
	se, ok := core.AsShipError(err)
	if !ok || se.Code != core.CodeGitIO {
		t.Fatalf("want CodeGitIO ShipError, got %v", err)
	}
}

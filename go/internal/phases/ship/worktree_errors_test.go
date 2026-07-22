//go:build integration

// worktree_errors_test.go — fault-injected coverage for shipFromWorktree
// runner-error branches (gitops.go:153-237) and writeShipBinding early
// error paths (gitops.go:268,287,291).
//
// Each test uses faultRunner to fail exactly one git subcommand against a
// genuine repo so the code executes all preceding steps via real git.
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

// --- gitops.go:153-154: worktree git add -A runner error -------------------

func TestShipFromWorktree_GitAddFails_Errors(t *testing.T) {
	repo, wt := makeWorktreeScenario(t)

	// Un-stage the file so "git add -A" is needed, then fail it.
	runGit(t, wt, "reset", "HEAD", "wt-change.txt")

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: worktree add fail",
		ProjectRoot:   repo,
		Runner:        faultRunner("git add", 1, nil),
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt)
	// cycle-1067: explicit-path staging (stageExplicitPaths) dropped the `-A`
	// from the message; the worktree stage-failure branch is unchanged.
	if err == nil || !strings.Contains(err.Error(), "git add failed") {
		t.Fatalf("want 'git add failed' error, got %v", err)
	}
}

// --- gitops.go:160-162: worktree diff --cached --quiet runner error ---------

func TestShipFromWorktree_DiffCachedQuietFails_Errors(t *testing.T) {
	repo, wt := makeWorktreeScenario(t)

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: diff quiet fail",
		ProjectRoot:   repo,
		Runner: func(ctx context.Context, name, cwd string, args, env []string,
			stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			// Fail specifically: git diff --cached --quiet (has both flags).
			if name == "git" && argsContain(args, "--cached") && argsContain(args, "--quiet") {
				return -1, errors.New("diff quiet injected error")
			}
			return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
		},
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt)
	if err == nil || !strings.Contains(err.Error(), "diff --cached --quiet failed") {
		t.Fatalf("want 'diff --cached --quiet failed' error, got %v", err)
	}
}

// --- gitops.go:167-169: rev-list runner error (ahead check) ----------------

func TestShipFromWorktree_RevListFails_WhenBranchAheadCheck(t *testing.T) {
	// Need worktreeCleanNoCommit == true (nothing to commit) but branch ahead.
	// Use DryRun=false with a clean worktree + branch ahead of main.
	repo := makeRepo(t)
	addRemote(t, repo)
	seedAudit(t, repo, "PASS")

	// Create worktree on a branch that is ahead of main by one commit.
	wt := tempRepoDir(t)
	runGit(t, repo, "worktree", "add", "-b", "cycle-ahead", wt)
	mustWrite(t, filepath.Join(wt, "ahead.txt"), "ahead\n")
	runGit(t, wt, "add", "ahead.txt")
	runGit(t, wt, "commit", "-m", "ahead commit")
	// Nothing else staged — worktreeCleanNoCommit will be true after add -A.

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: rev-list fail",
		ProjectRoot:   repo,
		Runner:        faultRunner("git rev-list", 1, errors.New("rev-list injected")),
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt)
	if err == nil {
		t.Fatal("want rev-list error, got nil")
	}
}

// --- gitops.go:181-183: buildDiffFooterAtDir error in worktree commit path -

func TestShipFromWorktree_BuildDiffFooterFails_Errors(t *testing.T) {
	repo, wt := makeWorktreeScenario(t)

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: diff footer fail",
		ProjectRoot:   repo,
		Runner: func(ctx context.Context, name, cwd string, args, env []string,
			stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			// buildDiffFooterAtDir calls "git -C <wt> diff --cached --name-status".
			// Match by --name-status (unique to this call).
			if name == "git" && argsContain(args, "--name-status") {
				return -1, errors.New("name-status injected error")
			}
			return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
		},
		Stdin:  strings.NewReader(""),
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt)
	if err == nil {
		t.Fatal("want buildDiffFooterAtDir error, got nil")
	}
	if !strings.Contains(err.Error(), "name-status") {
		t.Errorf("want name-status error, got %v", err)
	}
}

// --- gitops.go:227-229: ff-merge failure -----------------------------------

func TestShipFromWorktree_FFMergeFails_Errors(t *testing.T) {
	repo, wt := makeWorktreeScenario(t)

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: ff-merge fail",
		ProjectRoot:   repo,
		Runner:        faultRunner("git merge", 1, nil),
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt)
	if err == nil || !strings.Contains(err.Error(), "ff-merge") {
		t.Fatalf("want ff-merge error, got %v", err)
	}
}

// --- gitops.go:234-237: push failure ---------------------------------------

func TestShipFromWorktree_PushFails_Errors(t *testing.T) {
	repo, wt := makeWorktreeScenario(t)

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: push fail",
		ProjectRoot:   repo,
		Runner:        faultRunner("git push", 1, nil),
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt)
	if err == nil || !strings.Contains(err.Error(), "git push failed") {
		t.Fatalf("want push-failed error, got %v", err)
	}
}

// --- gitops.go:268-270: writeShipBinding readStateMap error ----------------

func TestWriteShipBinding_ReadStateMapError_ReturnsError(t *testing.T) {
	repo := makeRepo(t)

	// Make cycle-state.json a directory so readStateMap errors.
	csDir := filepath.Join(repo, ".evolve")
	if err := os.MkdirAll(csDir, 0o755); err != nil {
		t.Fatal(err)
	}
	csPath := filepath.Join(csDir, "cycle-state.json")
	if err := os.MkdirAll(csPath, 0o755); err != nil {
		t.Fatal(err)
	}

	opts := &Options{ProjectRoot: repo}
	err := writeShipBinding(opts, "abc123tree", "abc123sha")
	if err == nil {
		t.Fatal("want error from readStateMap on directory, got nil")
	}
}

// Note: writeShipBinding MkdirAll failure (gitops.go:291-293) is already
// covered by TestWriteShipBinding_MkdirFails_ReturnsError in coverage_final_test.go.
// TestWriteShipBinding_NoCycleID_Errors (no cycle_id) is in misc_gaps_test.go.

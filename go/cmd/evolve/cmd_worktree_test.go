package main

// cmd_worktree_test.go — RED contract for cycle-549's
// cli-command-layer-test-coverage task (triage-report.md top_n item, this
// lane's fleet_scope: cli-command-layer-test-coverage-worktree-swarm).
//
// PROBLEM: `evolve worktree create|list|cleanup` (cmd_worktree.go) had ZERO
// direct test coverage (0.0% per `go tool cover -func` on every handler) even
// though cycle-543 already lifted the sibling guardcmd/opscmd packages to the
// 80% bar — this file is the un-shipped remainder of the original inbox item
// (`cli-command-layer-test-coverage`), scoped by this lane's fleet_scope to
// cmd/evolve's worktree + swarm-reap surface.
//
// These tests drive the REAL `git worktree` subprocess against a throwaway
// repo in t.TempDir() — no fake Runner seam exists for this file today (the
// functions call exec.Command directly), so success+error coverage means
// actually creating/listing/removing a real worktree. Skips if `git` is
// unavailable (mirrors the existing e2e convention in this package, e.g.
// cmd_loop_coverage_test.go's TestRunLoop_ResumePhaseRunnerError).

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
}

// initRepo creates a git repo at dir with one commit on HEAD (worktree add
// needs a real commit to detach onto).
func initRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write f.txt: %v", err)
	}
	run("add", ".")
	run("commit", "-m", "init")
}

func TestRunWorktreeCreate_Success(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	initRepo(t, root)
	base := filepath.Join(root, ".evolve", "worktrees")

	var stdout, stderr strings.Builder
	code := runWorktreeCreate([]string{"--cycle", "1", "--project-root", root, "--base", base, "--lane", "testlane"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runWorktreeCreate exit=%d stderr=%s", code, stderr.String())
	}
	wt := strings.TrimSpace(stdout.String())
	if wt == "" {
		t.Fatal("runWorktreeCreate printed no worktree path")
	}
	if info, err := os.Stat(wt); err != nil || !info.IsDir() {
		t.Fatalf("worktree dir %q was not created: %v", wt, err)
	}
	// git itself must agree the worktree exists.
	out, err := exec.Command("git", "-C", root, "worktree", "list").CombinedOutput()
	if err != nil {
		t.Fatalf("git worktree list: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), wt) {
		t.Errorf("git worktree list = %s, want it to contain %q", out, wt)
	}
}

func TestRunWorktreeCreate_MissingCycle_Errors(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr strings.Builder
	code := runWorktreeCreate([]string{"--cycle", "0", "--project-root", root}, &stdout, &stderr)
	if code != 10 {
		t.Fatalf("exit = %d, want 10 (missing --cycle)", code)
	}
	if !strings.Contains(stderr.String(), "--cycle is required") {
		t.Errorf("stderr = %q, want it to mention --cycle is required", stderr.String())
	}
}

func TestRunWorktreeCreate_NotAGitRepo_ExitOne(t *testing.T) {
	requireGit(t)
	root := t.TempDir() // no `git init` — not a repo
	base := filepath.Join(root, ".evolve", "worktrees")

	var stdout, stderr strings.Builder
	code := runWorktreeCreate([]string{"--cycle", "1", "--project-root", root, "--base", base}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (git failure on a non-repo root); stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "git:") {
		t.Errorf("stderr = %q, want it to mention the git error", stderr.String())
	}
}

func TestRunWorktreeList_Success(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	initRepo(t, root)

	var stdout, stderr strings.Builder
	code := runWorktreeList([]string{"--project-root", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runWorktreeList exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), root) {
		t.Errorf("stdout = %q, want it to list the repo root %q", stdout.String(), root)
	}
}

func TestRunWorktreeList_NotAGitRepo_ExitOne(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	var stdout, stderr strings.Builder
	code := runWorktreeList([]string{"--project-root", root}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (not a git repo)", code)
	}
}

func TestRunWorktreeCleanup_RemovesCreatedWorktree(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	initRepo(t, root)
	base := filepath.Join(root, ".evolve", "worktrees")

	var createOut, createErr strings.Builder
	if code := runWorktreeCreate([]string{"--cycle", "2", "--project-root", root, "--base", base, "--lane", "testlane"}, &createOut, &createErr); code != 0 {
		t.Fatalf("setup: runWorktreeCreate exit=%d stderr=%s", code, createErr.String())
	}
	wt := strings.TrimSpace(createOut.String())

	var stdout, stderr strings.Builder
	code := runWorktreeCleanup([]string{"--cycle", "2", "--project-root", root, "--base", base, "--lane", "testlane"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runWorktreeCleanup exit=%d stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("worktree dir %q still exists after cleanup (err=%v)", wt, err)
	}
	out, _ := exec.Command("git", "-C", root, "worktree", "list").CombinedOutput()
	if strings.Contains(string(out), wt) {
		t.Errorf("git worktree list still shows removed worktree: %s", out)
	}
}

func TestRunWorktreeCleanup_PruneAll_ExitZero(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	initRepo(t, root)

	var stdout, stderr strings.Builder
	// cycle=0 (default) → prune-all path, not a targeted removal.
	code := runWorktreeCleanup([]string{"--project-root", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runWorktreeCleanup (prune) exit=%d stderr=%s", code, stderr.String())
	}
}

func TestRunWorktree_MissingSubcommand_ExitTen(t *testing.T) {
	var stdout, stderr strings.Builder
	code := runWorktree(nil, nil, &stdout, &stderr)
	if code != 10 {
		t.Fatalf("exit = %d, want 10 (missing subcommand)", code)
	}
}

func TestRunWorktree_UnknownSubcommand_ExitTen(t *testing.T) {
	var stdout, stderr strings.Builder
	code := runWorktree([]string{"bogus"}, nil, &stdout, &stderr)
	if code != 10 {
		t.Fatalf("exit = %d, want 10 (unknown subcommand)", code)
	}
	if !strings.Contains(stderr.String(), "unknown subcommand") {
		t.Errorf("stderr = %q, want it to mention unknown subcommand", stderr.String())
	}
}

func TestErrIsNotExist(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"os.ErrNotExist wrapped", errors.New("stat x: no such file or directory"), true},
		{"unrelated error", errors.New("permission denied"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := errIsNotExist(tc.err); got != tc.want {
				t.Errorf("errIsNotExist(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

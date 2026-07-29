package core

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

// worktree_lanebase_test.go — RED tests for cycle-1196 task
// `lane-base-fetch-origin-main` (todo id
// loop-must-base-lanes-on-origin-main-not-stale-local).
//
// gitWorktree.Create bases every new lane branch on the LOCAL HEAD of
// projectRoot (`git worktree add -B <branch> <wt> HEAD`, worktree.go:97) with
// zero fetch/origin interaction anywhere in the file. In a multi-lane fleet the
// local checkout drifts behind origin/main as sibling lanes land work, so each
// new lane silently forks from a stale tip — re-introducing already-fixed
// defects and inflating ship-time merge conflicts.
//
// Contract these tests pin:
//  1. origin exists  → fetch the remote tip in projectRoot BEFORE `worktree
//     add`, and cut the branch from the FETCHED ref (origin/main or
//     FETCH_HEAD), never the literal local "HEAD".
//  2. no origin      → no fetch, explicit local-HEAD fallback still succeeds
//     (isolated/local-only repos and test fixtures must not break).
//  3. fetch fails    → fail loudly: return a wrapped error and do NOT fall back
//     to the stale local tip (no `worktree add` at all).
//  4. reuse path     → an existing valid worktree is still reused with no fetch
//     and no add (regression guard on worktree.go:74-90).
//
// Uses the package gitRunner seam (git_seam_test.go / worktree_branchdelete_test.go
// style) — package core cannot import test/fixtures.FakeExec (import cycle).

// laneBaseFake scripts per-subcommand git responses: an origin-probe answer, a
// scriptable fetch outcome, and success for everything else. It records every
// call in order so the tests can assert on SEQUENCE (fetch strictly before
// worktree add), which a single-response recorder cannot express.
type laneBaseFake struct {
	calls     []gitCall
	hasOrigin bool
	fetchRC   int
	fetchErr  error
}

func (f *laneBaseFake) run(_ context.Context, name, dir string, args, _ []string, _ io.Reader, outw, errw io.Writer) (int, error) {
	f.calls = append(f.calls, gitCall{name: name, dir: dir, args: append([]string(nil), args...)})
	if len(args) == 0 {
		return 0, nil
	}
	switch args[0] {
	// Origin probe — accept any of the reasonable plumbing forms so the test
	// pins the BEHAVIOUR (does a remote exist?) and not one implementation.
	case "remote", "config", "ls-remote":
		if f.hasOrigin {
			if outw != nil {
				_, _ = outw.Write([]byte("origin\thttps://example.invalid/repo.git\n"))
			}
			return 0, nil
		}
		return 1, nil // no origin configured
	case "fetch":
		if f.fetchErr != nil {
			return -1, f.fetchErr
		}
		if f.fetchRC != 0 && errw != nil {
			_, _ = errw.Write([]byte("fatal: could not read from remote repository"))
		}
		return f.fetchRC, nil
	}
	return 0, nil
}

func useLaneBaseFake(t *testing.T, f *laneBaseFake) {
	t.Helper()
	orig := gitRunner
	gitRunner = f.run
	t.Cleanup(func() { gitRunner = orig })
}

// findCall returns the index of the first recorded call whose args start with
// the given prefix, or -1.
func (f *laneBaseFake) findCall(prefix ...string) int {
	for i, c := range f.calls {
		if len(c.args) < len(prefix) {
			continue
		}
		match := true
		for j, p := range prefix {
			if c.args[j] != p {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// laneBaseProvisioner returns a gitWorktree whose base is an absolute temp dir
// (Create refuses a relative base) plus a temp projectRoot.
func laneBaseProvisioner(t *testing.T) (gitWorktree, string) {
	t.Helper()
	return gitWorktree{baseOverride: filepath.Join(t.TempDir(), "worktrees")}, t.TempDir()
}

// TestGitWorktree_Create_FetchesOriginBeforeBasingLane is the headline
// criterion: with an origin remote configured, Create must fetch the upstream
// tip in projectRoot and cut the lane branch from that FETCHED ref, so a lane
// provisioned while the local checkout is behind origin/main still starts from
// current main.
//
// RED today: worktree.go:97 passes the literal "HEAD" and no fetch is ever run.
func TestGitWorktree_Create_FetchesOriginBeforeBasingLane(t *testing.T) {
	f := &laneBaseFake{hasOrigin: true}
	useLaneBaseFake(t, f)
	g, root := laneBaseProvisioner(t)

	wt, err := g.Create(root, 1196)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if wt == "" {
		t.Fatal("Create returned an empty worktree path")
	}

	fetchIdx := f.findCall("fetch")
	if fetchIdx < 0 {
		t.Fatalf("RED: Create never fetched the remote before basing the lane — calls=%+v", f.calls)
	}
	addIdx := f.findCall("worktree", "add")
	if addIdx < 0 {
		t.Fatalf("Create never ran `git worktree add` — calls=%+v", f.calls)
	}
	if fetchIdx > addIdx {
		t.Errorf("fetch ran at call %d, AFTER `worktree add` at %d — the fetch must precede basing or the lane still forks from the stale local tip", fetchIdx, addIdx)
	}

	fetch := f.calls[fetchIdx]
	if fetch.dir != root {
		t.Errorf("fetch dir = %q, want projectRoot %q (the fetch must update the shared object store the new worktree is cut from)", fetch.dir, root)
	}
	if !strings.Contains(strings.Join(fetch.args, " "), "origin") {
		t.Errorf("fetch args = %v, want a fetch of the `origin` remote", fetch.args)
	}

	add := f.calls[addIdx]
	if len(add.args) < 6 {
		t.Fatalf("worktree add args = %v, want [worktree add -B <branch> <wt> <start-ref>]", add.args)
	}
	startRef := add.args[len(add.args)-1]
	if startRef == "HEAD" {
		t.Errorf("RED: lane branch is still cut from the local %q — want the fetched upstream ref (origin/<default-branch> or FETCH_HEAD); args=%v", startRef, add.args)
	}
	if !strings.Contains(startRef, "origin/") && startRef != "FETCH_HEAD" {
		t.Errorf("lane start-ref = %q, want a fetched remote ref (contains \"origin/\" or FETCH_HEAD); args=%v", startRef, add.args)
	}
}

// TestGitWorktree_Create_NoOriginFallsBackToLocalHEAD is the edge/OOD case:
// isolated repos (local-only dev checkouts, several test fixtures) have no
// origin. Create must then skip the fetch entirely and keep the documented
// local-HEAD basing — a hard failure here would break every remoteless repo.
func TestGitWorktree_Create_NoOriginFallsBackToLocalHEAD(t *testing.T) {
	f := &laneBaseFake{hasOrigin: false}
	useLaneBaseFake(t, f)
	g, root := laneBaseProvisioner(t)

	wt, err := g.Create(root, 1196)
	if err != nil {
		t.Fatalf("RED: Create must still succeed in a repo with no origin remote (explicit local-HEAD fallback); got %v", err)
	}
	if wt == "" {
		t.Fatal("Create returned an empty worktree path")
	}

	if idx := f.findCall("fetch"); idx >= 0 {
		t.Errorf("Create fetched with no origin configured (call %d: %v) — the probe must gate the fetch", idx, f.calls[idx].args)
	}
	addIdx := f.findCall("worktree", "add")
	if addIdx < 0 {
		t.Fatalf("Create never ran `git worktree add` — calls=%+v", f.calls)
	}
	add := f.calls[addIdx]
	if got := add.args[len(add.args)-1]; got != "HEAD" {
		t.Errorf("no-origin lane start-ref = %q, want local %q (documented fallback); args=%v", got, "HEAD", add.args)
	}
}

// TestGitWorktree_Create_FetchFailureIsFatal is the NEGATIVE test and the
// strongest anti-no-op signal: an implementation that fetches "best-effort" and
// silently continues from the stale local tip reproduces the exact defect this
// task closes. With origin present and the fetch failing, Create must return a
// wrapped error and must NOT provision anything.
func TestGitWorktree_Create_FetchFailureIsFatal(t *testing.T) {
	f := &laneBaseFake{hasOrigin: true, fetchRC: 128}
	useLaneBaseFake(t, f)
	g, root := laneBaseProvisioner(t)

	wt, err := g.Create(root, 1196)
	if err == nil {
		t.Fatalf("RED: fetch failed (rc=128) but Create returned worktree %q and a nil error — a silent stale-local fallback is the defect, not the fix", wt)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "fetch") {
		t.Errorf("error %q does not name the failed fetch — the operator must be able to tell a fetch failure from a provisioning failure", err.Error())
	}
	if idx := f.findCall("worktree", "add"); idx >= 0 {
		t.Errorf("Create provisioned the lane anyway after a failed fetch (call %d: %v) — it must abort, not fork from the stale local tip", idx, f.calls[idx].args)
	}
}

// TestGitWorktree_Create_ReuseSkipsFetch guards the existing idempotent-reuse
// contract (worktree.go:74-90): a valid worktree for the cycle is returned
// as-is on resume/retry. Re-basing (or re-fetching) a live lane mid-cycle would
// discard in-progress work, so the fetch must be confined to the CREATE path.
func TestGitWorktree_Create_ReuseSkipsFetch(t *testing.T) {
	f := &laneBaseFake{hasOrigin: true}
	useLaneBaseFake(t, f)
	g, root := laneBaseProvisioner(t)

	// First call provisions; the fake git does not create the directory, but
	// linkGuardDeps does (go/bin + .evolve under the worktree), which is enough
	// for the os.Stat + rev-parse reuse probe to succeed on the second call.
	first, err := g.Create(root, 1196)
	if err != nil {
		t.Fatalf("Create (provision): %v", err)
	}
	f.calls = nil

	second, err := g.Create(root, 1196)
	if err != nil {
		t.Fatalf("Create (reuse): %v", err)
	}
	if second != first {
		t.Fatalf("reuse returned %q, want the existing worktree %q", second, first)
	}
	if idx := f.findCall("fetch"); idx >= 0 {
		t.Errorf("reuse path fetched (call %d: %v) — re-basing a live lane would discard in-progress work", idx, f.calls[idx].args)
	}
	if idx := f.findCall("worktree", "add"); idx >= 0 {
		t.Errorf("reuse path re-ran `worktree add` (call %d: %v) — reuse must be a no-op", idx, f.calls[idx].args)
	}
}

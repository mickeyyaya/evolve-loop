package main

// cmd_syncmain_test.go — RED tests (cycle 611, task sync-main-boundary-command)
// for the new `evolve sync-main` operator/boundary command (inbox
// ship-repair-merge-diverged-origin, weight 0.85): a recurring diverged-origin
// stall (3x on 2026-07-07) currently requires manual operator reconciliation.
//
// Contract the Builder implements (TDD-defined seam):
//
//	func runSyncMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int
//
// Registered in registry.go as the "sync-main" subcommand (mirrors
// runResetSHA's `--project-root` flag convention, cmd_resetsha.go).
//
// Preconditions (ALL must hold before any git mutation is attempted):
//   - no live run lease: read .evolve/cycle-state.json's workspace_path (if the
//     marker exists) and refuse if runlease.OwnerLive is true there
//   - clean index (git status --porcelain empty, ignoring .evolve/** per repo
//     .gitignore) — refuse on any uncommitted change
//   - cycle-state idle (see above)
//
// Behavior:
//   - fetch origin, then `git merge --no-edit origin/<branch>` on divergence
//   - a clean, non-conflicting divergence merges (a real merge commit, two
//     parents) — quiet tree, no operator involvement
//   - a conflicting divergence aborts cleanly: working tree and HEAD end up
//     EXACTLY as they started (no MERGE_HEAD, no conflict markers)
//   - NEVER rebases, force-pushes, or pushes — sync-main only ever moves local
//     history forward via merge; the bare origin ref must be byte-identical
//     before and after every scenario in this file
//
// RED now (undefined symbol runSyncMain → package main test build fails). Do
// NOT modify this file — implement the seam in a new cmd_syncmain.go.

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// --- git fixture helpers (sm* prefix avoids collision with other main tests) ---

func smGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = smFilteredEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v (dir=%s): %v\n%s", args, dir, err, out)
	}
	return string(out)
}

// smGitAllowFail runs git without failing the test — some fixture steps
// (e.g. a merge expected to conflict) legitimately exit non-zero.
func smGitAllowFail(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = smFilteredEnv()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func smFilteredEnv() []string {
	var out []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "EVOLVE_") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func smInitRepo(t *testing.T, dir string) {
	t.Helper()
	smGit(t, dir, "init", "-q")
	smGit(t, dir, "config", "user.email", "ci@example.com")
	smGit(t, dir, "config", "user.name", "ci")
	smGit(t, dir, "config", "commit.gpgsign", "false")
}

// smInitRepoWithRemote creates a local repo with one committed file
// ("base.txt"), a bare "origin" remote, and pushes main so both sides start
// identical. Returns (repoDir, bareRemoteDir).
func smInitRepoWithRemote(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	smInitRepo(t, repo)
	// .evolve/ (cycle-state.json, lease files) must never make the tree
	// "dirty" in the eyes of sync-main's precondition check.
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(".evolve/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	smGit(t, repo, "add", "-A")
	smGit(t, repo, "commit", "-q", "-m", "base")
	smGit(t, repo, "branch", "-M", "main")

	bare := t.TempDir()
	bare = filepath.Join(bare, "origin.git")
	smGit(t, t.TempDir(), "init", "-q", "--bare", bare)
	smGit(t, repo, "remote", "add", "origin", bare)
	smGit(t, repo, "push", "-q", "origin", "main")
	return repo, bare
}

// smRemoteCommit clones bare fresh, commits a new file, and pushes to
// origin/main — simulating a teammate's landed commit the local repo hasn't
// fetched yet.
func smRemoteCommit(t *testing.T, bare, filename, content string) {
	t.Helper()
	clone := t.TempDir()
	smGit(t, t.TempDir(), "clone", "-q", bare, clone)
	if err := os.WriteFile(filepath.Join(clone, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	smGit(t, clone, "add", "-A")
	smGit(t, clone, "commit", "-q", "-m", "remote: "+filename)
	smGit(t, clone, "push", "-q", "origin", "main")
}

func smHead(t *testing.T, dir string) string {
	t.Helper()
	return strings.TrimSpace(smGit(t, dir, "rev-parse", "HEAD"))
}

func smBareHead(t *testing.T, bare string) string {
	t.Helper()
	return strings.TrimSpace(smGit(t, bare, "rev-parse", "main"))
}

func smPorcelain(t *testing.T, dir string) string {
	t.Helper()
	return smGit(t, dir, "status", "--porcelain")
}

func smMergeHeadExists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git", "MERGE_HEAD"))
	return err == nil
}

func smWriteLiveLease(t *testing.T, repo string) {
	t.Helper()
	evolveDir := filepath.Join(repo, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wsDir := t.TempDir()
	if err := runlease.Write(wsDir, runlease.Lease{RunID: "sm-live-run"}, time.Now()); err != nil {
		t.Fatalf("runlease.Write: %v", err)
	}
	cs := map[string]any{"cycle_id": 5, "workspace_path": wsDir}
	b, _ := json.Marshal(cs)
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// AC: "Merges diverged origin on quiet tree" — a clean, non-conflicting
// divergence (local commit + a distinct remote commit) merges automatically
// into a real merge commit, and the merge NEVER pushes (bare ref unchanged).
func TestSyncMain_MergesQuietDivergedTree(t *testing.T) {
	repo, bare := smInitRepoWithRemote(t)
	smRemoteCommit(t, bare, "remote-change.txt", "from origin\n")

	if err := os.WriteFile(filepath.Join(repo, "local-change.txt"), []byte("from local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	smGit(t, repo, "add", "-A")
	smGit(t, repo, "commit", "-q", "-m", "local change")

	bareHeadBefore := smBareHead(t, bare)

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)
	if code != 0 {
		t.Fatalf("expected exit 0 on quiet divergence, got %d\nstdout=%s\nstderr=%s", code, out.String(), errb.String())
	}

	for _, f := range []string{"local-change.txt", "remote-change.txt", "base.txt"} {
		if _, err := os.Stat(filepath.Join(repo, f)); err != nil {
			t.Errorf("expected %s present after merge: %v", f, err)
		}
	}

	parents := strings.Fields(strings.TrimSpace(smGit(t, repo, "rev-list", "--parents", "-1", "HEAD")))
	if len(parents) != 3 {
		t.Errorf("expected a merge commit (1 hash + 2 parents), got %v", parents)
	}

	if got := smBareHead(t, bare); got != bareHeadBefore {
		t.Errorf("origin main ref changed — sync-main must never push (before=%s after=%s)", bareHeadBefore, got)
	}
	if p := smPorcelain(t, repo); p != "" {
		t.Errorf("expected clean tree after merge, got:\n%s", p)
	}
}

// AC: "refuses cleanly on ... dirty index" — an uncommitted change blocks the
// sync entirely; nothing is fetched/merged, HEAD and the dirty change survive
// untouched.
func TestSyncMain_RefusesOnDirtyIndex(t *testing.T) {
	repo, bare := smInitRepoWithRemote(t)
	smRemoteCommit(t, bare, "remote-change.txt", "from origin\n")

	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("line1\nDIRTY\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	headBefore := smHead(t, repo)

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)
	if code == 0 {
		t.Fatalf("expected refusal on dirty index, got exit 0\nstdout=%s", out.String())
	}
	if got := smHead(t, repo); got != headBefore {
		t.Errorf("HEAD must be unchanged on refusal: before=%s after=%s", headBefore, got)
	}
	b, err := os.ReadFile(filepath.Join(repo, "base.txt"))
	if err != nil || !strings.Contains(string(b), "DIRTY") {
		t.Errorf("dirty change must survive refusal untouched: %q err=%v", b, err)
	}
	if smMergeHeadExists(repo) {
		t.Error("no merge should have been attempted on a dirty index")
	}
}

// AC: "refuses cleanly on live lease" — an active cycle (fresh lease
// referenced by cycle-state.json) blocks the sync; HEAD is untouched.
func TestSyncMain_RefusesOnLiveLease(t *testing.T) {
	repo, bare := smInitRepoWithRemote(t)
	smRemoteCommit(t, bare, "remote-change.txt", "from origin\n")
	smWriteLiveLease(t, repo)

	headBefore := smHead(t, repo)

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)
	if code == 0 {
		t.Fatalf("expected refusal while a run lease is live, got exit 0\nstdout=%s", out.String())
	}
	if got := smHead(t, repo); got != headBefore {
		t.Errorf("HEAD must be unchanged when refused for a live lease: before=%s after=%s", headBefore, got)
	}
}

// AC: "refuses cleanly on conflict" — a genuinely conflicting divergence
// (same line edited on both sides) must abort back to the EXACT pre-merge
// state: no MERGE_HEAD, no conflict markers, HEAD unmoved, working tree
// unchanged. No auto-rebase escape hatch.
func TestSyncMain_RefusesCleanlyOnConflict(t *testing.T) {
	repo, bare := smInitRepoWithRemote(t)
	smRemoteCommit(t, bare, "base.txt", "line1\nremote-edit\nline3\n")

	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("line1\nlocal-edit\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	smGit(t, repo, "add", "-A")
	smGit(t, repo, "commit", "-q", "-m", "local conflicting edit")

	headBefore := smHead(t, repo)
	contentBefore, _ := os.ReadFile(filepath.Join(repo, "base.txt"))
	bareHeadBefore := smBareHead(t, bare)

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)
	if code == 0 {
		t.Fatalf("expected refusal on a genuine conflict, got exit 0\nstdout=%s", out.String())
	}
	if smMergeHeadExists(repo) {
		t.Error("conflict must be cleanly aborted — no lingering MERGE_HEAD")
	}
	if got := smHead(t, repo); got != headBefore {
		t.Errorf("HEAD must be unchanged after a clean conflict abort: before=%s after=%s", headBefore, got)
	}
	contentAfter, _ := os.ReadFile(filepath.Join(repo, "base.txt"))
	if string(contentAfter) != string(contentBefore) {
		t.Errorf("working tree must be restored verbatim after conflict abort:\nbefore=%q\nafter=%q", contentBefore, contentAfter)
	}
	if p := smPorcelain(t, repo); p != "" {
		t.Errorf("expected clean tree after conflict abort, got:\n%s", p)
	}
	if got := smBareHead(t, bare); got != bareHeadBefore {
		t.Errorf("origin main ref must never change, even on a conflict abort: before=%s after=%s", bareHeadBefore, got)
	}
}

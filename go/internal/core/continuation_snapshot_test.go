package core

// continuation_snapshot_test.go — ADR-0076 slice C (C1/C2): at the preserve
// decision a FAILed cycle's dirty worktree is SNAPSHOT-COMMITTED onto its
// cycle branch (an immutable ref — adoption never trusts mutable dirty state)
// and a continuation manifest is stamped into the workspace, gated on the
// carry-forward screen classifying the snapshot Clean against main.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

// initContinuationRepo creates a real repo with main checked out plus a cycle
// worktree via the REAL provisioner, returning (root, worktree).
func initContinuationRepo(t *testing.T, cycle int) (string, string) {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Mirror the production repo's ignore truth: linkGuardDeps plants
	// gitignored infrastructure (go/bin symlink, .evolve state links) in every
	// provisioned worktree — a snapshot must never see it as dirt.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".evolve/\ngo/bin/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "a.txt", ".gitignore")
	run("commit", "-m", "base")
	wt, err := gitWorktree{}.Create(root, cycle)
	if err != nil {
		t.Fatalf("provision worktree: %v", err)
	}
	return root, wt
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestSnapshotPreservedWorktree_CommitsDirtAndUntracked(t *testing.T) {
	_, wt := initContinuationRepo(t, 71)
	if err := os.WriteFile(filepath.Join(wt, "a.txt"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, "new_file.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	head := gitOut(t, wt, "rev-parse", "HEAD")

	sha, err := snapshotPreservedWorktree(context.Background(), wt)
	if err != nil {
		t.Fatalf("snapshotPreservedWorktree: %v", err)
	}
	if sha == head {
		t.Fatal("dirty worktree must produce a NEW snapshot commit")
	}
	if got := gitOut(t, wt, "status", "--porcelain"); got != "" {
		t.Errorf("worktree must be clean after snapshot; porcelain:\n%s", got)
	}
	if files := gitOut(t, wt, "show", "--name-only", "--format=", sha); !strings.Contains(files, "new_file.go") || !strings.Contains(files, "a.txt") {
		t.Errorf("snapshot must carry tracked edit AND untracked file; got:\n%s", files)
	}
}

func TestSnapshotPreservedWorktree_CleanIsIdempotentHEAD(t *testing.T) {
	_, wt := initContinuationRepo(t, 72)
	head := gitOut(t, wt, "rev-parse", "HEAD")
	sha, err := snapshotPreservedWorktree(context.Background(), wt)
	if err != nil {
		t.Fatalf("snapshotPreservedWorktree: %v", err)
	}
	if sha != head {
		t.Errorf("clean worktree snapshot = HEAD (%s), got %s", head, sha)
	}
}

func TestStampContinuationManifest_WritesGatedManifest(t *testing.T) {
	root, wt := initContinuationRepo(t, 73)
	ws := filepath.Join(root, ".evolve", "runs", "cycle-73")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, "feature.go"), []byte("package y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := gitOut(t, wt, "rev-parse", "HEAD")
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	cs := CycleState{CycleID: 73, WorkspacePath: ws, ActiveWorktree: wt, WorktreeBaseSHA: base}

	o.stampContinuationManifest(context.Background(), cs, 73, root)

	m, ok, err := continuation.ReadManifest(ws)
	if err != nil || !ok {
		t.Fatalf("manifest must exist after stamp: ok=%v err=%v", ok, err)
	}
	if m.Cycle != 73 || m.Worktree != wt || m.BaseSHA != base {
		t.Errorf("manifest fields: %+v", m)
	}
	if m.SnapshotSHA == "" || m.SnapshotSHA == base {
		t.Errorf("manifest must reference the NEW snapshot commit, got %q (base %q)", m.SnapshotSHA, base)
	}
	if m.Branch == "" {
		t.Error("manifest must carry the cycle branch")
	}
	// The snapshot ref must be reachable from the recorded branch (immutable
	// anchor even if the worktree directory is later pruned).
	if got := gitOut(t, root, "merge-base", "--is-ancestor", m.SnapshotSHA, m.Branch); got != "" {
		t.Errorf("snapshot not on branch: %s", got)
	}
}

func TestStampContinuationManifest_ConflictingWorkIsNotStamped(t *testing.T) {
	root, wt := initContinuationRepo(t, 74)
	ws := filepath.Join(root, ".evolve", "runs", "cycle-74")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	base := gitOut(t, wt, "rev-parse", "HEAD")
	// Diverge: main edits a.txt one way, the worktree another → 3-way conflict.
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("main-side\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, root, "add", "a.txt")
	gitOut(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "main-side")
	if err := os.WriteFile(filepath.Join(wt, "a.txt"), []byte("lane-side\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	cs := CycleState{CycleID: 74, WorkspacePath: ws, ActiveWorktree: wt, WorktreeBaseSHA: base}

	o.stampContinuationManifest(context.Background(), cs, 74, root)

	if _, ok, _ := continuation.ReadManifest(ws); ok {
		t.Error("conflicting work must NOT be stamped (the debugger path owns conflicts, not resumption)")
	}
}

// TestAbnormalEpilogue_StampsContinuation pins the G1 gap the FIRST live FAIL
// exposed (cycle-1078, 2026-07-23): a review-gate rejection exits RunCycle via
// the ERROR path, which never reaches finalizeCycle — the worktree was
// preserved but no continuation manifest was written, so the resumption
// machinery had nothing to bind. The unskippable abnormal epilogue must stamp
// too (idempotent with the finalize-path stamp; runs after the failure digest
// so FindingsPath has content).
func TestAbnormalEpilogue_StampsContinuation(t *testing.T) {
	root, wt := initContinuationRepo(t, 78)
	ws := filepath.Join(root, ".evolve", "runs", "cycle-78")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, "half_done.go"), []byte("package wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := gitOut(t, wt, "rev-parse", "HEAD")
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	cr := &cycleRun{
		o: o, ctx: context.Background(), cycle: 78,
		req: CycleRequest{ProjectRoot: root, GoalHash: "g"},
		cs:  CycleState{CycleID: 78, WorkspacePath: ws, ActiveWorktree: wt, WorktreeBaseSHA: base, Phase: "tdd"},
	}
	cr.cycleCompletedNormally = false

	cr.abnormalEpilogue()

	m, ok, err := continuation.ReadManifest(ws)
	if err != nil || !ok {
		t.Fatalf("abnormal epilogue must stamp the continuation manifest: ok=%v err=%v", ok, err)
	}
	if m.Cycle != 78 || m.SnapshotSHA == "" || m.SnapshotSHA == base {
		t.Errorf("manifest must reference a real snapshot commit: %+v", m)
	}
}

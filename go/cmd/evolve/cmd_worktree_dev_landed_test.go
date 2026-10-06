package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const bigFileLines = 20

func bigFile(changed map[int]string) string {
	var b strings.Builder
	for i := 1; i <= bigFileLines; i++ {
		line := fmt.Sprintf("line %d", i)
		if c, ok := changed[i]; ok {
			line = c
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

type landedHub struct {
	runtime, store string
	seed           *gittest.Repo
}

func newLandedHub(t *testing.T) landedHub {
	t.Helper()
	runtime, seed := devHubFixture(t)
	brCommit(t, seed.Dir, "big.txt", bigFile(nil), "big file")
	seed.Git("push", "-q", "origin", "main")
	return landedHub{runtime: runtime, store: filepath.Join(filepath.Dir(runtime), ".repo.git"), seed: seed}
}

func (h landedHub) dev(t *testing.T, task, branch string) string {
	t.Helper()
	if out, code := runDev(t, "create", "--dev", task, "--branch", branch, "--project-root", h.runtime); code != 0 {
		t.Fatalf("create --dev %s exit=%d\n%s", task, code, out)
	}
	return filepath.Join(filepath.Dir(h.runtime), "dev", task)
}

func (h landedHub) land(t *testing.T, name, content, msg string) {
	t.Helper()
	brCommit(t, h.seed.Dir, name, content, msg)
	h.seed.Git("push", "-q", "origin", "main")
}

func (h landedHub) fetch(t *testing.T) {
	t.Helper()
	brGit(t, h.store, "fetch", "-q", "origin", "+refs/heads/main:"+devOriginMain)
}

func (h landedHub) originMain(t *testing.T) string {
	t.Helper()
	return brOut(t, h.store, "rev-parse", devOriginMain)
}

func writeDevFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func noMergedPR(context.Context, string, string) (string, error) { return "", nil }

func cleanupOne(t *testing.T, h landedHub, task string, opts devCleanupOptions) (string, int) {
	t.Helper()
	if opts.mergedPRHeads == nil {
		opts.mergedPRHeads = noMergedPR
	}
	var out, errb bytes.Buffer
	code := runWorktreeCleanupDev(h.runtime, task, opts, &out, &errb)
	return out.String() + errb.String(), code
}

func assertDevGone(t *testing.T, h landedHub, dir, branch, out string) {
	t.Helper()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("%s survived its cleanup (stat err=%v)\n%s", dir, err, out)
	}
	if brBranchExists(t, h.store, branch) {
		t.Errorf("branch %s survived its cleanup\n%s", branch, out)
	}
}

func assertDevKept(t *testing.T, h landedHub, dir, branch, out string) {
	t.Helper()
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("%s was removed although it holds work not in origin/main: %v\n%s", dir, err, out)
	}
	if !brBranchExists(t, h.store, branch) {
		t.Errorf("branch %s was deleted although it holds work not in origin/main\n%s", branch, out)
	}
}

func TestWorktreeDevCleanup_ACherryPickedLandingIsCleaned(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	dir := h.dev(t, "t1", "b1")
	brCommit(t, dir, "feature.txt", "lane work\n", "lane: feature")
	brCommit(t, dir, "big.txt", bigFile(map[int]string{3: "lane edit"}), "lane: big edit")
	brGit(t, dir, "push", "-q", "origin", "HEAD:refs/heads/lane-b1")
	h.land(t, "other.txt", "someone else's landing\n", "main moves on")
	h.seed.Git("fetch", "-q", "origin", "lane-b1")
	h.seed.Git("cherry-pick", "FETCH_HEAD~1", "FETCH_HEAD")
	h.seed.Git("push", "-q", "origin", "main")

	out, code := cleanupOne(t, h, "t1", devCleanupOptions{})

	if code != 0 || !strings.Contains(out, "already in origin/main") {
		t.Fatalf("cleanup of a cherry-picked landing exit=%d, want 0 naming the content proof\n%s", code, out)
	}
	assertDevGone(t, h, dir, "b1", out)
}

func TestWorktreeDevCleanup_AnExtraUnlandedHunkIsRefused(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	dir := h.dev(t, "t1", "b1")
	brCommit(t, dir, "big.txt", bigFile(map[int]string{2: "landed hunk", 18: "unlanded hunk"}), "lane: two hunks")
	h.land(t, "big.txt", bigFile(map[int]string{2: "landed hunk"}), "only the first hunk lands")

	out, code := cleanupOne(t, h, "t1", devCleanupOptions{})

	if code != 1 || !strings.Contains(out, "not merged") || !strings.Contains(out, "big.txt") {
		t.Errorf("cleanup with an unlanded hunk exit=%d, want 1 naming the unmerged branch and big.txt\n%s", code, out)
	}
	assertDevKept(t, h, dir, "b1", out)
}

func TestWorktreeDevCleanup_AnUntrackedFileNotInMainIsRefused(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	dir := h.dev(t, "t1", "b1")
	brCommit(t, dir, "feature.txt", "lane work\n", "lane: feature")
	h.land(t, "feature.txt", "lane work\n", "feature (#9) squashed")
	writeDevFile(t, dir, "notes.txt", "never landed\n")
	writeDevFile(t, dir, "same.txt", "landed copy\n")
	h.land(t, "same.txt", "landed copy\n", "same.txt lands")

	out, code := cleanupOne(t, h, "t1", devCleanupOptions{})

	if code != 1 || !strings.Contains(out, "dirty") || !strings.Contains(out, "untracked notes.txt is not in origin/main") {
		t.Errorf("cleanup with an unlanded untracked file exit=%d, want 1 naming notes.txt\n%s", code, out)
	}
	assertDevKept(t, h, dir, "b1", out)
	if got := readDevFile(t, dir, "notes.txt"); got != "never landed\n" {
		t.Errorf("the untracked file changed: %q", got)
	}
}

func TestWorktreeDevCleanup_AnUntrackedFileThatDiffersFromMainIsRefused(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	dir := h.dev(t, "t1", "b1")
	h.land(t, "notes.txt", "main's version\n", "notes.txt lands differently")
	writeDevFile(t, dir, "notes.txt", "the lane's version\n")

	out, code := cleanupOne(t, h, "t1", devCleanupOptions{})

	if code != 1 || !strings.Contains(out, "untracked notes.txt differs from origin/main") {
		t.Errorf("cleanup with a differing untracked file exit=%d, want 1 naming notes.txt\n%s", code, out)
	}
	assertDevKept(t, h, dir, "b1", out)
}

func TestWorktreeDevCleanup_AStagedOnlyChangeAlreadyInMainIsCleaned(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	dir := h.dev(t, "t1", "b1")
	writeDevFile(t, dir, "feature.txt", "patch landed by the train\n")
	writeDevFile(t, dir, "big.txt", bigFile(map[int]string{7: "train edit"}))
	brGit(t, dir, "add", "feature.txt", "big.txt")
	h.land(t, "feature.txt", "patch landed by the train\n", "train: lane patch")
	h.land(t, "big.txt", bigFile(map[int]string{7: "train edit", 19: "later main edit"}), "train: lane patch, then main edits elsewhere")

	out, code := cleanupOne(t, h, "t1", devCleanupOptions{})

	if code != 0 || !strings.Contains(out, "already in origin/main") {
		t.Fatalf("cleanup of a staged-only landed change exit=%d, want 0\n%s", code, out)
	}
	assertDevGone(t, h, dir, "b1", out)
}

func TestWorktreeDevCleanup_TheDryRunRemovesNothingAndNeverFetches(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	dir := h.dev(t, "t1", "b1")
	writeDevFile(t, dir, "feature.txt", "patch landed by the train\n")
	h.land(t, "feature.txt", "patch landed by the train\n", "train: lane patch")
	h.fetch(t)
	fetched := h.originMain(t)
	h.land(t, "later.txt", "origin moves on\n", "origin moves on after the last fetch")

	out, code := cleanupOne(t, h, "t1", devCleanupOptions{dryRun: true})

	if code != 0 || !strings.Contains(out, "would remove") {
		t.Fatalf("dry run of a landed tree exit=%d, want 0 saying what it would remove\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "feature.txt")); err != nil {
		t.Errorf("the dry run removed the tree: %v", err)
	}
	if !brBranchExists(t, h.store, "b1") {
		t.Error("the dry run deleted the branch")
	}
	if got := h.originMain(t); got != fetched {
		t.Errorf("the dry run fetched: origin/main moved %s -> %s", fetched, got)
	}
}

func TestWorktreeDevCleanup_ADetachedOrUnregisteredTreeIsRefused(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	detached := h.dev(t, "t1", "b1")
	brGit(t, detached, "checkout", "-q", "--detach")
	plain := filepath.Join(filepath.Dir(h.runtime), "dev", "plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}

	if out, code := cleanupOne(t, h, "t1", devCleanupOptions{}); code != 1 || !strings.Contains(out, "not on a branch") {
		t.Errorf("cleanup of a detached tree exit=%d, want 1\n%s", code, out)
	}
	if out, code := cleanupOne(t, h, "plain", devCleanupOptions{}); code != 1 || !strings.Contains(out, "not a worktree of the hub store") {
		t.Errorf("cleanup of a directory that is not a hub worktree exit=%d, want 1\n%s", code, out)
	}
	if _, err := os.Stat(plain); err != nil {
		t.Errorf("the unregistered directory was removed: %v", err)
	}
}

func readDevFile(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func cleanupAll(t *testing.T, h landedHub, opts devCleanupOptions) (string, int) {
	t.Helper()
	if opts.mergedPRHeads == nil {
		opts.mergedPRHeads = noMergedPR
	}
	var out, errb bytes.Buffer
	code := runWorktreeCleanupDevAll(h.runtime, opts, &out, &errb)
	return out.String() + errb.String(), code
}

func TestWorktreeDevCleanupAll_CleansEveryLandedTaskAndNamesEachOneItKeeps(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	picked := h.dev(t, "picked", "b-picked")
	brCommit(t, picked, "picked.txt", "landed by cherry-pick\n", "lane: picked")
	staged := h.dev(t, "staged", "b-staged")
	writeDevFile(t, staged, "staged.txt", "landed by the train\n")
	brGit(t, staged, "add", "staged.txt")
	unlanded := h.dev(t, "unlanded", "b-unlanded")
	brCommit(t, unlanded, "big.txt", bigFile(map[int]string{12: "still in review"}), "lane: unlanded")
	h.land(t, "picked.txt", "landed by cherry-pick\n", "picked (#1)")
	h.land(t, "staged.txt", "landed by the train\n", "train (#2)")
	stray := filepath.Join(filepath.Dir(h.runtime), "dev", "stray")
	if err := os.MkdirAll(stray, 0o755); err != nil {
		t.Fatal(err)
	}

	out, code := cleanupAll(t, h, devCleanupOptions{now: time.Now().Add(3 * time.Hour)})

	if code != 0 {
		t.Fatalf("--all exit=%d, want 0: a kept task is a report, not a failure\n%s", code, out)
	}
	assertDevGone(t, h, picked, "b-picked", out)
	assertDevGone(t, h, staged, "b-staged", out)
	assertDevKept(t, h, unlanded, "b-unlanded", out)
	for _, want := range []string{"kept dev/unlanded:", "not merged", "kept dev/stray:", "not a worktree of the hub store", "2 removed, 2 kept, 0 failed"} {
		if !strings.Contains(out, want) {
			t.Errorf("--all output does not contain %q:\n%s", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(h.runtime, ".git")); err != nil {
		t.Errorf("the runtime plane was touched: %v", err)
	}
}

func TestWorktreeDevCleanupAll_KeepsATreeChangedWithinTheQuietPeriod(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	fresh := h.dev(t, "fresh", "b-fresh")

	out, code := cleanupAll(t, h, devCleanupOptions{})

	if code != 0 || !strings.Contains(out, "kept dev/fresh:") || !strings.Contains(out, "changed within the last 2h0m0s") {
		t.Errorf("--all on a tree created moments ago exit=%d, want 0 keeping it as active\n%s", code, out)
	}
	assertDevKept(t, h, fresh, "b-fresh", out)
}

func TestWorktreeDevCleanupAll_TheDryRunRemovesNothing(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	staged := h.dev(t, "staged", "b-staged")
	writeDevFile(t, staged, "staged.txt", "landed by the train\n")
	brGit(t, staged, "add", "staged.txt")
	h.land(t, "staged.txt", "landed by the train\n", "train (#2)")
	h.fetch(t)

	out, code := cleanupAll(t, h, devCleanupOptions{dryRun: true, now: time.Now().Add(3 * time.Hour)})

	if code != 0 || !strings.Contains(out, "would remove dev/staged") || !strings.Contains(out, "1 would be removed, 0 kept, 0 failed") {
		t.Fatalf("--all --dry-run exit=%d\n%s", code, out)
	}
	assertDevKept(t, h, staged, "b-staged", out)
}

func TestWorktreeCleanup_DevAllAndDryRunFlags(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	h.dev(t, "t1", "b1")
	cases := []struct {
		name string
		args []string
		code int
		want string
	}{
		{"bare --dev before --all selects every task", []string{"cleanup", "--dev", "--all", "--dry-run"}, 0, "would be removed"},
		{"--all alone needs --dev", []string{"cleanup", "--all"}, 10, "--all needs --dev"},
		{"a task and --all together", []string{"cleanup", "--dev", "t1", "--all"}, 10, "--all"},
		{"--dry-run needs --dev", []string{"cleanup", "--stale", "--dry-run"}, 10, "--dry-run needs --dev"},
		{"bare --dev with nothing after it", []string{"cleanup", "--dev"}, 10, "--dev"},
	}
	for _, c := range cases {
		out, code := runDev(t, append(c.args, "--project-root", h.runtime)...)
		if code != c.code || !strings.Contains(out, c.want) {
			t.Errorf("%s: exit=%d, want %d naming %q\n%s", c.name, code, c.code, c.want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(h.runtime), "dev", "t1")); err != nil {
		t.Errorf("a flag-parsing case removed dev/t1: %v", err)
	}
}

func TestWorktreeDevCleanup_ATreeThatChangesAfterItsProofIsKept(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	dir := h.dev(t, "t1", "b1")
	writeDevFile(t, dir, "feature.txt", "patch landed by the train\n")
	h.land(t, "feature.txt", "patch landed by the train\n", "train: lane patch")
	h.fetch(t)
	c, refused := newDevCleaner(h.runtime, devCleanupOptions{mergedPRHeads: noMergedPR}, 0)
	if refused.code != devRemovable {
		t.Fatalf("newDevCleaner refused: %+v", refused)
	}
	j, o := c.judge(dir)
	if o.code != devRemovable {
		t.Fatalf("judge of a landed tree = %+v, want removable", o)
	}

	writeDevFile(t, dir, "late.txt", "written after the proof\n")
	o = c.remove(dir, j)

	if o.code != devKept || !strings.Contains(o.detail, "changed while it was being checked") {
		t.Errorf("remove after a late write = %+v, want kept naming the change", o)
	}
	if got := readDevFile(t, dir, "late.txt"); got != "written after the proof\n" {
		t.Errorf("the late write was lost: %q", got)
	}
	assertDevKept(t, h, dir, "b1", o.detail)
}

func repeatedBlocksFile(first, second string) string {
	block := func(mid string) string {
		return "ctx one\nctx two\nctx three\n" + mid + "\nctx four\nctx five\nctx six\n"
	}
	var b strings.Builder
	b.WriteString("start\n" + block(first))
	for i := 1; i <= 8; i++ {
		fmt.Fprintf(&b, "between %d\n", i)
	}
	b.WriteString(block(second) + "end\n")
	return b.String()
}

func TestWorktreeDevCleanup_AnEditMatchingAnIdenticalBlockElsewhereInMainIsNotLanded(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	h.land(t, "repeated.txt", repeatedBlocksFile("OLD", "NEW"), "a file with two near-identical blocks")
	dir := h.dev(t, "t1", "b1")
	writeDevFile(t, dir, "repeated.txt", repeatedBlocksFile("NEW", "NEW"))

	out, code := cleanupOne(t, h, "t1", devCleanupOptions{})

	if code != 1 || !strings.Contains(out, "repeated.txt") {
		t.Errorf("an uncommitted edit whose text already exists elsewhere in main's copy exit=%d, want 1 naming repeated.txt: the first block's change is not in origin/main\n%s", code, out)
	}
	if got := readDevFile(t, dir, "repeated.txt"); got != repeatedBlocksFile("NEW", "NEW") {
		t.Errorf("the uncommitted edit was lost: %q", got)
	}
}

func TestWorktreeDevCleanup_ADeletionOrModeChangeNotInMainIsRefused(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	brCommit(t, h.seed.Dir, "tool.sh", "echo tool\n", "a script")
	h.seed.Git("push", "-q", "origin", "main")
	deleted := h.dev(t, "del", "b-del")
	if err := os.Remove(filepath.Join(deleted, "big.txt")); err != nil {
		t.Fatal(err)
	}
	chmodded := h.dev(t, "mode", "b-mode")
	if err := os.Chmod(filepath.Join(chmodded, "tool.sh"), 0o755); err != nil {
		t.Fatal(err)
	}

	if out, code := cleanupOne(t, h, "del", devCleanupOptions{}); code != 1 || !strings.Contains(out, "big.txt") {
		t.Errorf("a deletion main does not have exit=%d, want 1 naming big.txt\n%s", code, out)
	}
	if out, code := cleanupOne(t, h, "mode", devCleanupOptions{}); code != 1 || !strings.Contains(out, "tool.sh") {
		t.Errorf("a mode change main does not have exit=%d, want 1 naming tool.sh\n%s", code, out)
	}
	h.land(t, "keep.txt", "unrelated\n", "main moves on")
	h.seed.Git("rm", "-q", "big.txt")
	h.seed.Git("commit", "-q", "-m", "big.txt goes")
	h.seed.Git("push", "-q", "origin", "main")
	if out, code := cleanupOne(t, h, "del", devCleanupOptions{}); code != 0 {
		t.Errorf("a deletion main also made exit=%d, want 0\n%s", code, out)
	}
}

func TestWorktreeDevCleanup_ACleanTreeThatGainsACommitAfterItsProofIsKept(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	dir := h.dev(t, "t1", "b1")
	brCommit(t, dir, "feature.txt", "lane work\n", "lane: feature")
	h.land(t, "feature.txt", "lane work\n", "feature (#9) squashed")
	h.fetch(t)
	c, refused := newDevCleaner(h.runtime, devCleanupOptions{mergedPRHeads: noMergedPR}, 0)
	if refused.code != devRemovable {
		t.Fatalf("newDevCleaner refused: %+v", refused)
	}
	j, o := c.judge(dir)
	if o.code != devRemovable || j.dirty {
		t.Fatalf("judge of a clean, content-landed tree = %+v dirty=%v, want removable and clean", o, j.dirty)
	}

	brCommit(t, dir, "late.txt", "committed after the proof\n", "lane: late commit")
	o = c.remove(dir, j)

	if o.code != devKept {
		t.Errorf("remove after a late commit = %+v, want kept", o)
	}
	assertDevKept(t, h, dir, "b1", o.detail)
}

func ageAdminFiles(t *testing.T, dir string, age time.Duration) {
	t.Helper()
	admin := brOut(t, dir, "rev-parse", "--absolute-git-dir")
	then := time.Now().Add(-age)
	for _, f := range []string{"HEAD", "index", filepath.Join("logs", "HEAD")} {
		if err := os.Chtimes(filepath.Join(admin, f), then, then); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWorktreeDevCleanupAll_AFreshlyEditedFileKeepsATreeWhoseGitFilesAreOld(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	dir := h.dev(t, "q", "b-q")
	h.land(t, "fresh.txt", "already in main\n", "fresh.txt lands")
	writeDevFile(t, dir, "fresh.txt", "already in main\n")
	ageAdminFiles(t, dir, 3*time.Hour)

	out, code := cleanupAll(t, h, devCleanupOptions{})

	if code != 0 || !strings.Contains(out, "kept dev/q:") || !strings.Contains(out, "changed within the last") {
		t.Errorf("--all on a tree whose only fresh change is a file edited moments ago exit=%d, want it kept as active: the quiet period clocks edited files, not only git's own files\n%s", code, out)
	}
	assertDevKept(t, h, dir, "b-q", out)
}

func TestWorktreeDevCleanupAll_TheQuietPeriodComesFromThePlanesPolicy(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	dir := h.dev(t, "staged", "b-staged")
	writeDevFile(t, dir, "staged.txt", "landed by the train\n")
	brGit(t, dir, "add", "staged.txt")
	h.land(t, "staged.txt", "landed by the train\n", "train (#2)")
	if err := os.MkdirAll(filepath.Join(h.runtime, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeDevFile(t, filepath.Join(h.runtime, ".evolve"), "policy.json", `{"gc":{"worktrees":{"dev_quiet_minutes":600}}}`)

	out, code := cleanupAll(t, h, devCleanupOptions{dryRun: true, now: futureNow()})

	if code != 0 || !strings.Contains(out, "kept dev/staged:") || !strings.Contains(out, "changed within the last 10h0m0s") {
		t.Errorf("--all with gc.worktrees.dev_quiet_minutes=600 on a tree changed 3h ago exit=%d, want it kept for the policy's 10h\n%s", code, out)
	}
}

func TestWorktreeDevCleanupAll_AFreshlyEditedFileWithANonASCIINameKeepsTheTree(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	dir := h.dev(t, "q", "b-q")
	h.land(t, "日本.txt", "already in main\n", "日本.txt lands")
	writeDevFile(t, dir, "日本.txt", "already in main\n")
	ageAdminFiles(t, dir, 3*time.Hour)

	out, code := cleanupAll(t, h, devCleanupOptions{})

	if code != 0 || !strings.Contains(out, "kept dev/q:") || !strings.Contains(out, "changed within the last") {
		t.Errorf("--all on a tree whose only fresh change is a just-written file git quotes in porcelain exit=%d, want it kept as active\n%s", code, out)
	}
	assertDevKept(t, h, dir, "b-q", out)
}

func TestLastChange_AnUnreadablePathIsAnErrorNotAnOldTree(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDevFile(t, locked, "f.txt", "x\n")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	if _, err := os.Lstat(filepath.Join(locked, "f.txt")); err == nil {
		t.Skip("a 0o000 directory is searchable here (running as root)")
	}

	if _, err := lastChange(root, []string{"locked/f.txt", "gone.txt"}); err == nil {
		t.Error("lastChange swallowed a permission error: a path it cannot stat must fail the quiet check closed, while a deleted path (gone.txt) is simply skipped")
	}
	if got, err := lastChange(root, []string{"gone.txt"}); err != nil || !got.IsZero() {
		t.Errorf("lastChange of a deleted path = %v, %v; want zero time and no error", got, err)
	}
}

func TestWorktreeDevCleanupAll_AFreshUnstagedEditToATrackedFileKeepsTheTree(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	dir := h.dev(t, "q", "b-q")
	h.land(t, "big.txt", bigFile(map[int]string{5: "landed edit"}), "big.txt edit lands")
	writeDevFile(t, dir, "big.txt", bigFile(map[int]string{5: "landed edit"}))
	ageAdminFiles(t, dir, 3*time.Hour)

	out, code := cleanupAll(t, h, devCleanupOptions{})

	if code != 0 || !strings.Contains(out, "kept dev/q:") || !strings.Contains(out, "changed within the last") {
		t.Errorf("--all on a tree whose only fresh change is an unstaged edit to a tracked file exit=%d, want it kept as active\n%s", code, out)
	}
	assertDevKept(t, h, dir, "b-q", out)
}

package wtcheckpoint_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

func save(t *testing.T, w wtcheckpoint.Worktree, o wtcheckpoint.SaveOptions) wtcheckpoint.SaveResult {
	t.Helper()
	res, err := wtcheckpoint.Save(context.Background(), w, o)
	if err != nil {
		t.Fatalf("Save(%s): %v", w.Dir, err)
	}
	return res
}

func TestSave_RecordsTheStagedTreeUnderTheFullWorkingTree(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	dirtyThreeWays(t, h, w.Dir)
	head := h.git(w.Dir, "rev-parse", "HEAD")

	res := save(t, w, wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060846Z")})

	if res.Status != wtcheckpoint.StatusSaved || res.Ref != "refs/checkpoints/task/20261006T060846Z" {
		t.Fatalf("Save = %+v, want saved at refs/checkpoints/task/20261006T060846Z", res)
	}
	full, staged := res.Ref, res.Ref+"^"
	if got := h.git(w.Dir, "rev-parse", staged+"^"); got != head {
		t.Errorf("the staged commit's parent = %s, want HEAD %s", got, head)
	}
	if got := h.git(w.Dir, "ls-tree", "-r", "--name-only", staged); strings.Contains(got, "untracked.txt") || !strings.Contains(got, "staged.txt") {
		t.Errorf("staged tree paths = %q, want staged.txt and no untracked file", got)
	}
	if got := h.git(w.Dir, "show", staged+":a.txt"); got != "a" {
		t.Errorf("staged a.txt = %q, want HEAD's content: the unstaged edit is not staged", got)
	}
	if got := h.git(w.Dir, "show", full+":a.txt"); got != "a edited, unstaged" {
		t.Errorf("full a.txt = %q, want the working-tree edit", got)
	}
	if got := h.git(w.Dir, "show", full+":notes/untracked.txt"); got != "untracked" {
		t.Errorf("full untracked file = %q, want it recorded", got)
	}
	if got := h.git(w.Dir, "log", "--format=%s", "-2", full); got != "checkpoint task worktree 20261006T060846Z\ncheckpoint task staged 20261006T060846Z" {
		t.Errorf("subjects = %q, want the console snapshot's subjects", got)
	}
	if status, _, _, _ := gitIn(w.Dir, "status", "--porcelain"); status != " M a.txt\nA  staged.txt\n?? notes/\n" {
		t.Errorf("the worktree's own status changed: %q", status)
	}
}

func TestSave_NeverTouchesTheRealIndex(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	dirtyThreeWays(t, h, w.Dir)
	index := h.git(w.Dir, "rev-parse", "--path-format=absolute", "--git-path", "index")
	before, beforeInfo := readWithInfo(t, index)

	save(t, w, wtcheckpoint.SaveOptions{Keep: 20, Label: "probe", Now: clockAt("20261006T060846Z")})

	after, afterInfo := readWithInfo(t, index)
	if !bytes.Equal(before, after) {
		t.Error("the worktree's index bytes changed: a save must work on a copy, never the real index")
	}
	if !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
		t.Errorf("the worktree's index mtime moved from %v to %v", beforeInfo.ModTime(), afterInfo.ModTime())
	}
	if _, err := os.Stat(index + ".lock"); !os.IsNotExist(err) {
		t.Errorf("an index.lock was left behind: %v", err)
	}
}

func TestSave_NeverIncludesTheBinaryOrTheLedger(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	writeFile(t, w.Dir, "go/evolve", "binary-v2-rebuilt\n")
	writeFile(t, w.Dir, ".evolve/ledger.jsonl", "{}\n")
	writeFile(t, w.Dir, ".evolve/ledger.tip", "abc\n")
	writeFile(t, w.Dir, ".evolve/state.json", "{}\n")

	res := save(t, w, wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060846Z")})

	if res.Status != wtcheckpoint.StatusSaved {
		t.Fatalf("status = %s, want saved (state.json is a real change)", res.Status)
	}
	if got, want := h.git(w.Dir, "rev-parse", res.Ref+":go/evolve"), h.git(w.Dir, "rev-parse", "HEAD:go/evolve"); got != want {
		t.Errorf("go/evolve in the snapshot = %s, want HEAD's blob %s: the rebuilt binary must not be captured", got, want)
	}
	paths := h.git(w.Dir, "ls-tree", "-r", "--name-only", res.Ref)
	if strings.Contains(paths, "ledger") {
		t.Errorf("the snapshot carries a ledger file: %q", paths)
	}
	if !strings.Contains(paths, ".evolve/state.json") {
		t.Errorf("the snapshot lost an ordinary untracked file: %q", paths)
	}
}

func TestSave_SkipsAnUnchangedTreeAndACleanOne(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	dirty := h.addWorktree("dev/dirty", "feat/dirty")
	clean := h.addWorktree("dev/clean", "feat/clean")
	writeFile(t, dirty.Dir, "a.txt", "changed\n")
	writeFile(t, clean.Dir, "ignored.log", "ignored files are not work\n")
	o := wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060846Z", "20261006T070000Z")}

	first := save(t, dirty, o)
	second := save(t, dirty, o)
	cleanRes := save(t, clean, o)

	if first.Status != wtcheckpoint.StatusSaved {
		t.Fatalf("first save = %s, want saved", first.Status)
	}
	if second.Status != wtcheckpoint.StatusUnchanged || second.Ref != first.Ref {
		t.Errorf("second save = %+v, want unchanged pointing at %s", second, first.Ref)
	}
	if cleanRes.Status != wtcheckpoint.StatusClean || cleanRes.Ref != "" {
		t.Errorf("clean save = %+v, want clean with no ref", cleanRes)
	}
	if refs := lines(h.git(h.store.Dir, "for-each-ref", "--format=%(refname)", "refs/checkpoints/")); len(refs) != 1 {
		t.Errorf("refs = %v, want exactly the first save's", refs)
	}
}

func TestSave_KeepsOnlyTheNewestNPerWorktree(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	other := h.addWorktree("dev/other", "feat/other")
	writeFile(t, other.Dir, "a.txt", "other\n")
	save(t, other, wtcheckpoint.SaveOptions{Keep: 1, Now: clockAt("20261006T050000Z")})
	o := wtcheckpoint.SaveOptions{Keep: 2, Now: clockAt("20261006T060000Z", "20261006T070000Z", "20261006T080000Z")}

	var last wtcheckpoint.SaveResult
	for _, body := range []string{"one\n", "two\n", "three\n"} {
		writeFile(t, w.Dir, "a.txt", body)
		last = save(t, w, o)
	}

	got := lines(h.git(h.store.Dir, "for-each-ref", "--format=%(refname)", "refs/checkpoints/"))
	want := []string{"refs/checkpoints/other/20261006T050000Z", "refs/checkpoints/task/20261006T070000Z", "refs/checkpoints/task/20261006T080000Z"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("refs = %v, want %v", got, want)
	}
	if strings.Join(last.Pruned, ",") != "refs/checkpoints/task/20261006T060000Z" {
		t.Errorf("Pruned = %v, want the oldest of task only", last.Pruned)
	}
}

func TestSave_SucceedsWhileAnotherProcessHoldsTheIndexLock(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	dirtyThreeWays(t, h, w.Dir)
	lock := h.git(w.Dir, "rev-parse", "--path-format=absolute", "--git-path", "index") + ".lock"
	if err := os.WriteFile(lock, []byte("another process is mid git add"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := save(t, w, wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060846Z")})

	if res.Status != wtcheckpoint.StatusSaved {
		t.Fatalf("status = %s, want saved while index.lock is held", res.Status)
	}
	if got, err := os.ReadFile(lock); err != nil || string(got) != "another process is mid git add" {
		t.Errorf("the other process's index.lock = %q, %v: a save must never take, rewrite or remove it", got, err)
	}
}

func TestSave_AConcurrentGitAddNeverFails(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	dirtyThreeWays(t, h, w.Dir)
	const rounds = 15
	var wg sync.WaitGroup
	addErrs := make(chan error, rounds)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := os.MkdirAll(filepath.Join(w.Dir, "adds"), 0o755); err != nil {
			addErrs <- err
			return
		}
		for i := 0; i < rounds; i++ {
			name := filepath.Join(w.Dir, "adds", "f"+string(rune('a'+i))+".txt")
			if err := os.WriteFile(name, []byte("x\n"), 0o644); err != nil {
				addErrs <- err
				return
			}
			if _, stderr, code, err := gitIn(w.Dir, "add", "-A", "adds"); err != nil || code != 0 {
				addErrs <- fmt.Errorf("git add rc=%d err=%v: %s", code, err, stderr)
			}
		}
	}()
	for i := 0; i < rounds; i++ {
		if _, err := wtcheckpoint.Save(context.Background(), w, wtcheckpoint.SaveOptions{Keep: 50, Now: uniqueClock(i)}); err != nil {
			t.Errorf("save %d failed beside a concurrent git add: %v", i, err)
		}
	}
	wg.Wait()
	close(addErrs)
	for err := range addErrs {
		t.Errorf("the concurrent git add failed while saves ran: %v", err)
	}
}

func readWithInfo(t *testing.T, p string) ([]byte, os.FileInfo) {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return b, info
}

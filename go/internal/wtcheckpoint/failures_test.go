package wtcheckpoint_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

func TestSave_FailsLoudlyOnAnUnmergedIndexAndRecordsNothing(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	other := h.addWorktree("dev/other", "feat/other")
	writeFile(t, other.Dir, "a.txt", "theirs\n")
	h.git(other.Dir, "commit", "-qam", "theirs")
	writeFile(t, w.Dir, "a.txt", "ours\n")
	h.git(w.Dir, "commit", "-qam", "ours")
	if _, _, code, _ := gitIn(w.Dir, "merge", "-q", "feat/other"); code == 0 {
		t.Fatal("the fixture merge did not conflict")
	}

	_, err := wtcheckpoint.Save(context.Background(), w, wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060000Z")})

	if err == nil {
		t.Fatal("Save of a worktree with unmerged paths succeeded")
	}
	if got := refsIn(t, h); got != "" {
		t.Errorf("refs after a failed save = %s, want none", got)
	}
}

func TestList_RefusesARefThatIsNotACheckpoint(t *testing.T) {
	t.Parallel()
	for name, ref := range map[string]string{
		"no stamp":      "refs/checkpoints/task/latest",
		"no base above": "refs/checkpoints/task/20261006T060000Z",
	} {
		h := newHub(t)
		h.store.Git("update-ref", ref, "main")
		if _, err := wtcheckpoint.List(context.Background(), h.hub(), ""); err == nil {
			t.Errorf("%s: List accepted %s pointing at a plain commit", name, ref)
		}
	}
	h := newHub(t)
	blob := strings.TrimSpace(h.store.Git("rev-parse", "main:a.txt"))
	h.store.Git("update-ref", "refs/checkpoints/task/20261006T060000Z", blob)
	if _, err := wtcheckpoint.List(context.Background(), h.hub(), "task"); err == nil {
		t.Error("List accepted a checkpoint ref that names a blob")
	}
}

func TestPrune_LandedFailsOnACheckpointWithoutABaseAndDeletesNothing(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	h.store.Git("update-ref", "refs/checkpoints/task/20261006T060000Z", "main")

	if _, err := wtcheckpoint.Prune(context.Background(), h.hub(), wtcheckpoint.PruneOptions{Keep: 20, Landed: true}); err == nil {
		t.Error("Prune --landed judged a ref with no base")
	}
	if got := refsIn(t, h); got != "refs/checkpoints/task/20261006T060000Z" {
		t.Errorf("refs left = %s, want it kept", got)
	}
}

func TestPush_FailsLoudlyWithoutAnOrigin(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	writeFile(t, w.Dir, "a.txt", "work\n")
	saved := save(t, w, wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060000Z")})

	if err := wtcheckpoint.Push(context.Background(), h.hub(), []string{saved.Ref}); err == nil || !strings.Contains(err.Error(), "git push origin") {
		t.Errorf("Push with no origin err = %v, want the git push failure", err)
	}
	if err := wtcheckpoint.Push(context.Background(), h.hub(), nil); err != nil {
		t.Errorf("Push of nothing = %v, want a no-op", err)
	}
}

func TestRestore_NeverOverwritesAnIgnoredFileInTheTarget(t *testing.T) {
	t.Parallel()
	for name, rel := range map[string]string{"an ignored file": "build.log", "a file in an ignored directory": "out/report.txt"} {
		h := newHub(t)
		w := h.addWorktree("dev/task", "feat/task")
		writeFile(t, w.Dir, rel, "force-added work\n")
		h.git(w.Dir, "add", "-f", rel)
		saved := save(t, w, wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060000Z")})
		target := h.addWorktree("dev/target", "feat/target")
		writeFile(t, target.Dir, rel, "the target's own ignored file\n")

		_, err := wtcheckpoint.Restore(context.Background(), h.hub(), saved.Ref, target.Dir)

		if err == nil || !strings.Contains(err.Error(), "refused") || !strings.Contains(err.Error(), rel) {
			t.Errorf("%s: Restore err = %v, want a refusal naming %s", name, err, rel)
		}
		if got, _ := os.ReadFile(filepath.Join(target.Dir, rel)); string(got) != "the target's own ignored file\n" {
			t.Errorf("%s: the target's ignored file = %q, want it untouched", name, got)
		}
	}
}

func TestSave_RefusesAWorktreeNameThatCannotNameARef(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/bad..name", "feat/bad")
	writeFile(t, w.Dir, "a.txt", "work\n")

	_, err := wtcheckpoint.Save(context.Background(), w, wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060000Z")})

	if err == nil || !strings.Contains(err.Error(), "refused") {
		t.Errorf("Save of %q err = %v, want a refusal", w.Name, err)
	}
}

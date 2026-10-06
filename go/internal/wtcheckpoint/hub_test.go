package wtcheckpoint_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

func TestResolveHub_FindsTheSharedStoreFromAnyWorktree(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	hub, err := wtcheckpoint.ResolveHub(w.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if realDir(t, hub.Store) != realDir(t, h.store.Dir) || realDir(t, hub.Root) != realDir(t, h.root) {
		t.Errorf("hub = %+v, want store %s under root %s", hub, h.store.Dir, h.root)
	}
	if _, err := wtcheckpoint.ResolveHub(t.TempDir()); err == nil {
		t.Error("ResolveHub accepted a directory outside any repository")
	}
}

func TestHub_WorktreeRefusesATargetThatIsNotALinkedWorktree(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	hub := h.hub()
	if err := os.MkdirAll(filepath.Join(w.Dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := gittest.Fixture(t)
	foreign.Git("commit", "-q", "--allow-empty", "-m", "x")
	foreignLinked := filepath.Join(t.TempDir(), "foreign-linked")
	foreign.Git("worktree", "add", "-q", "--detach", foreignLinked)

	for name, dir := range map[string]string{
		"a plain directory":                   t.TempDir(),
		"the store itself":                    h.store.Dir,
		"a subdirectory of a linked worktree": filepath.Join(w.Dir, "sub"),
		"another repository's main worktree":  foreign.Dir,
		"another repository's linked one":     foreignLinked,
	} {
		if got, err := hub.Worktree(context.Background(), dir); err == nil || !strings.Contains(err.Error(), "refused") {
			t.Errorf("%s: Worktree(%s) = %+v, %v; want a refusal", name, dir, got, err)
		}
	}
	got, err := hub.Worktree(context.Background(), w.Dir)
	if err != nil || got.Name != "task" || realDir(t, got.Dir) != realDir(t, w.Dir) {
		t.Errorf("Worktree(linked) = %+v, %v; want task at %s", got, err, w.Dir)
	}
}

func TestHub_DevWorktreesNeverIncludeTheRuntimePlaneOrACycleWorktree(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	runtime := filepath.Join(h.root, "runtime")
	h.store.Git("worktree", "add", "-q", runtime, "main")
	h.store.Git("worktree", "add", "-q", "--detach", filepath.Join(runtime, ".evolve", "worktrees", "cycle-7"), "main")
	h.addWorktree("dev/a", "feat/a")
	h.addWorktree("dev/b", "feat/b")
	gone := h.addWorktree("dev/gone", "feat/gone")
	if err := os.RemoveAll(gone.Dir); err != nil {
		t.Fatal(err)
	}

	got, err := h.hub().DevWorktrees(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, w := range got {
		names = append(names, w.Name)
	}
	if strings.Join(names, ",") != "a,b" {
		t.Errorf("DevWorktrees = %v, want only the live dev/ trees a,b (no runtime, cycle-7, console or a removed one)", names)
	}
}

func realDir(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

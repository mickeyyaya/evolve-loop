package gc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

type devEnv struct {
	t       *testing.T
	store   *gittest.Repo
	root    string
	runtime string
	evolve  string
	real    time.Time
}

func newDevEnv(t *testing.T) *devEnv {
	t.Helper()
	src := gittest.Fixture(t)
	commitRepoFile(t, src, "base.txt", "base\n")
	store := gittest.Bare(t)
	src.Git("push", "-q", store.Dir, "main")
	store.Git("update-ref", "refs/remotes/origin/main", "main")
	root := filepath.Dir(store.Dir)
	runtime := filepath.Join(root, "runtime")
	store.Git("worktree", "add", "-q", "--detach", runtime, "main")
	return &devEnv{t: t, store: store, root: root, runtime: runtime, evolve: filepath.Join(runtime, ".evolve"), real: time.Now()}
}

func (e *devEnv) addDev(name string) string {
	e.t.Helper()
	dir := filepath.Join(e.root, "dev", name)
	e.store.Git("worktree", "add", "-q", "-b", "br-"+name, dir, "main")
	return dir
}

func (e *devEnv) commitIn(dir, rel, body string) {
	e.t.Helper()
	writeRepoFile(e.t, dir, rel, body)
	e.store.Git("-C", dir, "add", "-A")
	e.store.Git("-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "work "+rel)
}

func (e *devEnv) commitDatedIn(dir, rel, body string, at time.Time) {
	e.t.Helper()
	writeRepoFile(e.t, dir, rel, body)
	e.store.Git("-C", dir, "add", "-A")
	stamp := strconv.FormatInt(at.Unix(), 10) + " +0000"
	e.t.Setenv("GIT_COMMITTER_DATE", stamp)
	e.t.Setenv("GIT_AUTHOR_DATE", stamp)
	e.store.Git("-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "work "+rel)
	os.Unsetenv("GIT_COMMITTER_DATE")
	os.Unsetenv("GIT_AUTHOR_DATE")
	e.store.Git("-C", dir, "reset", "-q", "--soft", "HEAD")
}

func (e *devEnv) options(offset time.Duration) DevOptions {
	return DevOptions{
		ProjectRoot: e.runtime,
		EvolveDir:   e.evolve,
		Now:         func() time.Time { return e.real.Add(offset) },
		Exec:        sysexec.DefaultRunner,
		Processes:   func(context.Context) ([]Process, error) { return nil, nil },
		OpenPRs:     noOpenPRs,
	}
}

var noOpenPRs OpenPRLookup = func(context.Context, string) (map[string][]time.Time, error) { return nil, nil }

var failingDevSave DevSaveFunc = func(context.Context, wtcheckpoint.Worktree) (wtcheckpoint.SaveResult, error) {
	return wtcheckpoint.SaveResult{}, errors.New("disk full")
}

func archiveActions(m DevManifest) []DevAction {
	var out []DevAction
	for _, it := range m.Items {
		out = append(out, it.Action)
	}
	return out
}

func (e *devEnv) plan(o DevOptions) DevManifest {
	e.t.Helper()
	m, err := PlanDevWorktrees(context.Background(), o)
	if err != nil {
		e.t.Fatalf("PlanDevWorktrees: %v", err)
	}
	return m
}

func (e *devEnv) hub() wtcheckpoint.Hub {
	e.t.Helper()
	h, err := wtcheckpoint.ResolveHub(e.runtime)
	if err != nil {
		e.t.Fatal(err)
	}
	return h
}

func devItemFor(t *testing.T, m DevManifest, dir string) DevItem {
	t.Helper()
	for _, it := range m.Items {
		if resolvePath(it.Path) == resolvePath(dir) {
			return it
		}
	}
	t.Fatalf("no item for %s in %+v", dir, m.Items)
	return DevItem{}
}

const idleOffset = 20 * 24 * time.Hour

func (e *devEnv) touchIndex(dir string, at time.Time) {
	e.t.Helper()
	gitDir := e.store.Git("-C", dir, "rev-parse", "--absolute-git-dir")
	if err := os.Chtimes(filepath.Join(gitDir, "index"), at, at); err != nil {
		e.t.Fatal(err)
	}
}

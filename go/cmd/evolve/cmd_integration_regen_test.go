package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/derived"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

const (
	regenIndex = "docs/research-index.md"
	regenShard = "docs/research-index"
	regenHead  = "# Research index\n<!-- GENERATED:research-index BEGIN -->\n"
	regenTail  = "<!-- GENERATED:research-index END -->\n"
)

func regenAggregate(sections ...string) string {
	return regenHead + strings.Join(sections, "") + regenTail
}

func shardGenerator(dir string) derived.Worktree {
	w := core.DerivedWorktree(dir)
	w.Run = func(_ context.Context, args ...string) error {
		if slices.Equal(args, []string{"docs", "index", "check"}) {
			return nil
		}
		shards, err := filepath.Glob(filepath.Join(dir, regenShard, "*.md"))
		if err != nil {
			return err
		}
		slices.Sort(shards)
		var sections []string
		for _, p := range shards {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			sections = append(sections, string(b))
		}
		return os.WriteFile(filepath.Join(dir, regenIndex), []byte(regenAggregate(sections...)), 0o644)
	}
	return w
}

type regenFixture struct {
	g     *gittest.Repo
	onto  string
	tree  string
	regen derivedRegen
}

func newRegenFixture(t *testing.T) regenFixture {
	t.Helper()
	g := gittest.Fixture(t)
	g.Write(regenShard+"/a.md", "alpha\n")
	g.Write(regenIndex, regenAggregate("alpha\n"))
	g.Git("add", ".")
	g.Git("commit", "-q", "-m", "base")
	onto := g.Git("rev-parse", "HEAD")
	g.Write(regenShard+"/b.md", "bravo\n")
	g.Write(regenShard+"/c.md", "charlie\n")
	g.Write(regenIndex, regenHead+"alpha\n<<<<<<< ours\nbravo\n=======\ncharlie\n>>>>>>> theirs\n"+regenTail)
	g.Git("add", "-A")
	tree := g.Git("write-tree")
	g.Git("reset", "-q", "--hard", onto)
	g.Git("clean", "-fdq")
	dir := filepath.Join(t.TempDir(), "integration-regen")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return regenFixture{g: g, onto: onto, tree: tree, regen: derivedRegen{Repo: integration.Repo{Dir: g.Dir, Git: integration.SysGit()}, Dir: dir, Worktree: shardGenerator}}
}

func (f regenFixture) leftovers(t *testing.T) {
	t.Helper()
	if list := f.g.Git("worktree", "list", "--porcelain"); strings.Count(list, "worktree ") != 1 {
		t.Errorf("git worktree list shows an entry of the port:\n%s", list)
	}
	if entries, err := os.ReadDir(f.regen.Dir); len(entries) != 0 || (err != nil && !errors.Is(err, os.ErrNotExist)) {
		t.Errorf("the port left %v in %s (%v)", entries, f.regen.Dir, err)
	}
}

func failingVerb(real integration.GitFunc, verb ...string) integration.GitFunc {
	return func(ctx context.Context, dir, stdin string, args ...string) (string, error) {
		if len(args) >= len(verb) && slices.Equal(args[:len(verb)], verb) {
			return "", errors.New("the " + strings.Join(verb, " ") + " verb failed")
		}
		return real(ctx, dir, stdin, args...)
	}
}

func TestDerivedRegen_RegeneratesTheConflictedTreeIntoOneCleanTree(t *testing.T) {
	t.Parallel()
	f := newRegenFixture(t)
	tree, err := f.regen.Regen(context.Background(), f.onto, f.tree, []string{regenIndex})
	if err != nil {
		t.Fatalf("Regen: %v", err)
	}
	if got, want := f.g.Git("cat-file", "-p", tree+":"+regenIndex), strings.TrimSpace(regenAggregate("alpha\n", "bravo\n", "charlie\n")); got != want {
		t.Errorf("the aggregate is %q, want %q", got, want)
	}
	f.leftovers(t)
}

func TestDerivedRegen_OnlyTheDeclaredOutputsLandInTheTree(t *testing.T) {
	t.Parallel()
	f := newRegenFixture(t)
	f.regen.Worktree = func(dir string) derived.Worktree {
		w := shardGenerator(dir)
		generate := w.Run
		w.Run = func(ctx context.Context, args ...string) error {
			if err := os.WriteFile(filepath.Join(dir, "generator.log"), []byte("stray\n"), 0o644); err != nil {
				return err
			}
			return generate(ctx, args...)
		}
		return w
	}
	tree, err := f.regen.Regen(context.Background(), f.onto, f.tree, []string{regenIndex})
	if err != nil {
		t.Fatalf("Regen: %v", err)
	}
	if listed := f.g.Git("ls-tree", "-r", "--name-only", tree); strings.Contains(listed, "generator.log") {
		t.Errorf("a file outside the declared outputs landed in the tree:\n%s", listed)
	}
	f.leftovers(t)
}

func TestDerivedRegen_ALeftoverMarkerIsAnErrorAndLeavesNoWorktree(t *testing.T) {
	t.Parallel()
	f := newRegenFixture(t)
	f.regen.Worktree = func(dir string) derived.Worktree {
		w := core.DerivedWorktree(dir)
		w.Run = func(context.Context, ...string) error { return nil }
		return w
	}
	_, err := f.regen.Regen(context.Background(), f.onto, f.tree, []string{regenIndex})
	if err == nil || !strings.Contains(err.Error(), "leftover conflict marker") {
		t.Fatalf("Regen = %v, want the leftover marker error", err)
	}
	f.leftovers(t)
}

func TestDerivedRegen_AFailedStepReturnsTheErrorAndLeavesNoWorktree(t *testing.T) {
	t.Parallel()
	real := integration.SysGit()
	cases := []struct {
		name  string
		setup func(f *regenFixture) (onto, tree string, paths []string)
		want  string
	}{
		{"prune", func(f *regenFixture) (string, string, []string) {
			f.regen.Repo.Git = failingVerb(real, "worktree", "prune")
			return f.onto, f.tree, []string{regenIndex}
		}, "worktree prune verb failed"},
		{"the directory", func(f *regenFixture) (string, string, []string) {
			f.regen.Dir = filepath.Join(f.regen.Dir, "absent")
			return f.onto, f.tree, []string{regenIndex}
		}, "no such file"},
		{"add the worktree", func(f *regenFixture) (string, string, []string) {
			return strings.Repeat("1", 40), f.tree, []string{regenIndex}
		}, "worktree"},
		{"read the tree", func(f *regenFixture) (string, string, []string) {
			return f.onto, strings.Repeat("2", 40), []string{regenIndex}
		}, "read-tree"},
		{"a path with no generator", func(f *regenFixture) (string, string, []string) {
			return f.onto, f.tree, []string{"a.go"}
		}, "a.go"},
		{"the generator", func(f *regenFixture) (string, string, []string) {
			f.regen.Worktree = func(dir string) derived.Worktree {
				w := core.DerivedWorktree(dir)
				w.Run = func(context.Context, ...string) error { return errors.New("generator did not build") }
				return w
			}
			return f.onto, f.tree, []string{regenIndex}
		}, "generator did not build"},
		{"stage", func(f *regenFixture) (string, string, []string) {
			f.regen.Repo.Git = failingVerb(real, "add")
			f.regen.Worktree = func(dir string) derived.Worktree {
				w := shardGenerator(dir)
				w.Git = func(ctx context.Context, args ...string) (string, int, error) { return "", 0, nil }
				return w
			}
			return f.onto, f.tree, []string{regenIndex}
		}, "add verb failed"},
		{"write the tree", func(f *regenFixture) (string, string, []string) {
			f.regen.Repo.Git = failingVerb(real, "write-tree")
			return f.onto, f.tree, []string{regenIndex}
		}, "write-tree verb failed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := newRegenFixture(t)
			onto, tree, paths := c.setup(&f)
			if _, err := f.regen.Regen(context.Background(), onto, tree, paths); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("Regen = %v, want an error with %q", err, c.want)
			}
			f.leftovers(t)
		})
	}
}

func TestDerivedRegen_ACancelledBatchContextStillRemovesTheWorktree(t *testing.T) {
	t.Parallel()
	f := newRegenFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	f.regen.Worktree = func(dir string) derived.Worktree {
		w := core.DerivedWorktree(dir)
		w.Run = func(context.Context, ...string) error {
			cancel()
			return ctx.Err()
		}
		return w
	}
	if _, err := f.regen.Regen(ctx, f.onto, f.tree, []string{regenIndex}); err == nil {
		t.Fatal("Regen = nil, want the cancelled generator error")
	}
	f.leftovers(t)
}

func TestIntegrationRegen_TheWiringCreatesTheDirectoryAndBindsTheGenerator(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var warn bytes.Buffer
	env := integrationEnv{Git: integration.SysGit(), Root: root, Derived: shardGenerator}
	port := env.regen(integration.Repo{Dir: root, Git: env.Git}, root, &warn)
	if want := filepath.Join(root, ".evolve", "ephemeral", "integration-regen"); port.Dir != want {
		t.Errorf("Dir = %q, want %q", port.Dir, want)
	}
	if info, err := os.Stat(port.Dir); err != nil || !info.IsDir() {
		t.Errorf("the wiring did not create %s: %v", port.Dir, err)
	}
	if port.Worktree == nil || warn.Len() != 0 {
		t.Errorf("worktree=%v warn=%q", port.Worktree != nil, warn.String())
	}
	if (integrationEnv{}).regenPort(integration.Repo{}, root, &warn) != nil {
		t.Error("an env with no generator must give no port, so that a derived conflict ejects as before")
	}
	if env.regenPort(integration.Repo{Dir: root, Git: env.Git}, root, &warn) == nil {
		t.Error("an env with a generator must give a port")
	}
	blocker := filepath.Join(root, "blocked")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	warn.Reset()
	env.regen(integration.Repo{Dir: blocker}, blocker, &warn)
	if !strings.Contains(warn.String(), "integration-regen") {
		t.Errorf("an uncreatable directory must warn, got %q", warn.String())
	}
}

func newShardJourney(t *testing.T) *trainJourney {
	t.Helper()
	j := newTrainJourney(t)
	j.work.Write(regenShard+"/a.md", "alpha\n")
	j.work.Write(regenIndex, regenAggregate("alpha\n"))
	j.work.Git("add", ".")
	j.work.Git("commit", "-q", "-m", "shards")
	j.work.Git("push", "-q", "origin", "main")
	j.gh.main = mainRun{tip: j.originMain(), status: "completed", conclusion: "success"}
	j.env.Derived = shardGenerator
	return j
}

func (j *trainJourney) prFiles(t *testing.T, n int, message string, files map[string]string) {
	t.Helper()
	name := "pr-" + strconv.Itoa(n)
	j.work.Git("checkout", "-q", "-b", name, "main")
	for p, body := range files {
		j.work.Write(p, body)
	}
	j.work.Git("add", ".")
	j.work.Git("commit", "-q", "-m", message)
	j.work.Git("push", "-q", "origin", name+":refs/pull/"+strconv.Itoa(n)+"/head")
	j.work.Git("checkout", "-q", "main")
}

func TestJourneyConsolePRTrainComposesTwoShardPRsWithNoEjection(t *testing.T) {
	t.Parallel()
	j := newShardJourney(t)
	j.prFiles(t, 1, "docs: charlie", map[string]string{regenShard + "/c.md": "charlie\n", regenIndex: regenAggregate("alpha\n", "charlie\n")})
	j.prFiles(t, 2, "docs: bravo", map[string]string{regenShard + "/b.md": "bravo\n", regenIndex: regenAggregate("alpha\n", "bravo\n")})
	for _, n := range []string{"1", "2"} {
		if rc, _, errb := j.run("record-pr", "--pr", n, "--class", "docs-review"); rc != 0 {
			t.Fatalf("record-pr %s: rc=%d %s", n, rc, errb)
		}
	}
	before := j.originMain()
	rc, out, errb := j.run("batch", "--dry-run", "1", "2")
	if rc != 0 || j.originMain() != before {
		t.Fatalf("dry run rc=%d err=%s", rc, errb)
	}
	if strings.Contains(out, "eject") || !strings.Contains(out, "batch: 2 PRs") {
		t.Errorf("the dry run must compose both shard PRs with no ejection:\n%s", out)
	}
}

func TestRegenEntries_OneEntryOwnsManyOutputsAndAppearsOnce(t *testing.T) {
	t.Parallel()
	entries, err := regenEntries([]string{"commands/x.md", ".codex-plugin/plugin.json", regenIndex})
	if err != nil {
		t.Fatalf("regenEntries: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name)
	}
	if !slices.Equal(names, []string{"skill-projections", "research-index"}) {
		t.Errorf("entries = %v, want each owner once in path order", names)
	}
}

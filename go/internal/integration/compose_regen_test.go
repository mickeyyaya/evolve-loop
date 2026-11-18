package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/overlap"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	indexFile  = "docs/research-index.md"
	shardDir   = "docs/research-index"
	indexHead  = "# Research index\n<!-- GENERATED:research-index BEGIN -->\n"
	indexTail  = "<!-- GENERATED:research-index END -->\n"
	goFileBase = "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"
)

func aggregate(sections ...string) string {
	return indexHead + strings.Join(sections, "") + indexTail
}

func newRegenFixture(t *testing.T) composeFixture {
	t.Helper()
	g := gittest.Fixture(t)
	g.Write(shardDir+"/a.md", "alpha\n")
	g.Write(indexFile, aggregate("alpha\n"))
	g.Write("a.go", goFileBase)
	g.Git("add", ".")
	g.Git("commit", "-q", "-m", "base")
	return composeFixture{g: g, repo: Repo{Dir: g.Dir, Git: SysGit()}, base: headOf(g)}
}

type regenCall struct {
	onto  string
	tree  string
	paths []string
}

type shardRegen struct {
	t     *testing.T
	repo  Repo
	mu    sync.Mutex
	calls []regenCall
	fail  error
}

func (s *shardRegen) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func (s *shardRegen) regen(ctx context.Context, onto, tree string, paths []string) (string, error) {
	s.mu.Lock()
	s.calls = append(s.calls, regenCall{onto, tree, paths})
	s.mu.Unlock()
	if s.fail != nil {
		return "", s.fail
	}
	dir := filepath.Join(s.t.TempDir(), "wt")
	git := func(args ...string) string {
		out, err := s.repo.Git(ctx, dir, "", args...)
		if err != nil {
			s.t.Errorf("regen stub: %v", err)
		}
		return strings.TrimSpace(out)
	}
	if _, err := s.repo.Git(ctx, s.repo.Dir, "", "worktree", "add", "--detach", dir, onto); err != nil {
		return "", err
	}
	defer func() { _, _ = s.repo.Git(ctx, s.repo.Dir, "", "worktree", "remove", "--force", dir) }()
	git("read-tree", "-u", "--reset", tree)
	shards, err := filepath.Glob(filepath.Join(dir, shardDir, "*.md"))
	if err != nil {
		return "", err
	}
	slices.Sort(shards)
	var sections []string
	for _, p := range shards {
		b, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		sections = append(sections, string(b))
	}
	if err := os.WriteFile(filepath.Join(dir, indexFile), []byte(aggregate(sections...)), 0o644); err != nil {
		return "", err
	}
	git("add", "-A")
	return git("write-tree"), nil
}

func (f composeFixture) shardLane(t *testing.T, cycle int, shard, section string, existing ...string) DeliverableRecord {
	t.Helper()
	sections := append(slices.Clone(existing), section)
	slices.Sort(sections)
	return f.lane(t, cycle, map[string]string{shardDir + "/" + shard + ".md": section, indexFile: aggregate(append([]string{"alpha\n"}, sections...)...)})
}

func regening(f composeFixture, s *shardRegen) Composer {
	var port RegenFunc = s.regen
	return Composer{Repo: f.repo, Stage: policy.FleetIntegrationShadow, Regen: port}
}

func ejectionOfErr(t *testing.T, err error) *Ejection {
	t.Helper()
	var ej *Ejection
	if !errors.As(err, &ej) {
		t.Fatalf("err = %v, want an *Ejection", err)
	}
	return ej
}

func TestCompose_TwoShardBranchesComposeWithNoEjectionAndTheAggregateKeepsShardOrder(t *testing.T) {
	t.Parallel()
	f := newRegenFixture(t)
	first := f.shardLane(t, 601, "c", "charlie\n")
	second := f.shardLane(t, 602, "b", "bravo\n")
	s := &shardRegen{t: t, repo: f.repo}
	comp := regening(f, s)
	got, err := comp.Compose(context.Background(), f.batch(t), first)
	if err != nil {
		t.Fatalf("Compose(first): %v", err)
	}
	got, err = comp.Compose(context.Background(), got.Batch, second)
	if err != nil {
		t.Fatalf("Compose(second): %v", err)
	}
	if want := aggregate("alpha\n", "bravo\n", "charlie\n"); f.g.Git("show", got.Batch.Tip+":"+indexFile) != strings.TrimSpace(want) {
		t.Errorf("the aggregate is %q, want both sections in shard order", f.g.Git("show", got.Batch.Tip+":"+indexFile))
	}
	if s.count() != 1 {
		t.Fatalf("the port ran %d times, want 1", s.count())
	}
	if c := s.calls[0]; c.onto != f.g.Git("rev-parse", "refs/heads/"+got.Batch.Branch+"^") || !slices.Equal(c.paths, []string{indexFile}) || len(c.tree) != 40 {
		t.Errorf("port call = %+v, want the running tip, a tree and [%s]", c, indexFile)
	}
}

func TestCompose_AMixedConflictEjectsAndNeverCallsThePort(t *testing.T) {
	t.Parallel()
	f := newRegenFixture(t)
	first := f.lane(t, 611, map[string]string{shardDir + "/c.md": "charlie\n", indexFile: aggregate("alpha\n", "charlie\n"), "a.go": "ONE\n" + goFileBase[4:]})
	second := f.lane(t, 612, map[string]string{shardDir + "/b.md": "bravo\n", indexFile: aggregate("alpha\n", "bravo\n"), "a.go": "uno\n" + goFileBase[4:]})
	s := &shardRegen{t: t, repo: f.repo}
	comp := regening(f, s)
	got, err := comp.Compose(context.Background(), f.batch(t), first)
	if err != nil {
		t.Fatal(err)
	}
	_, err = comp.Compose(context.Background(), got.Batch, second)
	ej := ejectionOfErr(t, err)
	if ej.Cause != CauseConflict || !slices.Equal(ej.Paths, []string{"a.go", indexFile}) {
		t.Errorf("ejection = %+v, want cause conflict on a.go and the index", ej)
	}
	if s.count() != 0 {
		t.Errorf("the port ran %d times on a mixed conflict, want 0", s.count())
	}
}

func TestCompose_ADeletedAggregateIsNotDerivedAndEjectsWithoutThePort(t *testing.T) {
	t.Parallel()
	f := newRegenFixture(t)
	first := f.shardLane(t, 621, "c", "charlie\n")
	f.g.Git("checkout", "-q", "-b", "lane-622", f.base)
	f.g.Git("rm", "-q", indexFile)
	f.g.Git("commit", "-q", "-m", "delete the aggregate")
	sha := headOf(f.g)
	f.g.Git("checkout", "-q", "main")
	patch, err := f.repo.PatchID(context.Background(), sha)
	if err != nil {
		t.Fatal(err)
	}
	second := DeliverableRecord{Wave: fixtureWave, Cycle: 622, Trailer: CycleTrailer(622), Commits: []CommitIdentity{{SHA: sha, BaseSHA: f.base, PatchID: patch}}, Audit: first.Audit}
	s := &shardRegen{t: t, repo: f.repo}
	comp := regening(f, s)
	got, err := comp.Compose(context.Background(), f.batch(t), first)
	if err != nil {
		t.Fatal(err)
	}
	_, err = comp.Compose(context.Background(), got.Batch, second)
	ej := ejectionOfErr(t, err)
	if ej.Cause != CauseConflict || !slices.Equal(ej.Paths, []string{indexFile}) {
		t.Errorf("ejection = %+v, want cause conflict on the index", ej)
	}
	if s.count() != 0 {
		t.Errorf("the port ran %d times on a deleted aggregate, want 0", s.count())
	}
}

func TestCompose_AnUnreadableBlobIsNotDerivedAndEjectsWithoutThePort(t *testing.T) {
	t.Parallel()
	f := newRegenFixture(t)
	first := f.shardLane(t, 691, "c", "charlie\n")
	second := f.shardLane(t, 692, "b", "bravo\n")
	s := &shardRegen{t: t, repo: f.repo}
	comp := regening(f, s)
	real := comp.Repo.Git
	comp.Repo.Git = func(ctx context.Context, dir, stdin string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "cat-file" {
			return "", errors.New("cat-file failed")
		}
		return real(ctx, dir, stdin, args...)
	}
	got, err := comp.Compose(context.Background(), f.batch(t), first)
	if err != nil {
		t.Fatal(err)
	}
	_, err = comp.Compose(context.Background(), got.Batch, second)
	if ej := ejectionOfErr(t, err); ej.Cause != CauseConflict || s.count() != 0 {
		t.Errorf("ejection = %+v, port ran %d times; want cause conflict and no port", ej, s.count())
	}
}

func TestCompose_ANilPortEjectsADerivedConflictAsBefore(t *testing.T) {
	t.Parallel()
	f := newRegenFixture(t)
	first := f.shardLane(t, 631, "c", "charlie\n")
	second := f.shardLane(t, 632, "b", "bravo\n")
	comp := Composer{Repo: f.repo, Stage: policy.FleetIntegrationShadow}
	got, err := comp.Compose(context.Background(), f.batch(t), first)
	if err != nil {
		t.Fatal(err)
	}
	_, err = comp.Compose(context.Background(), got.Batch, second)
	if ej := ejectionOfErr(t, err); ej.Cause != CauseConflict || !slices.Equal(ej.Paths, []string{indexFile}) {
		t.Errorf("ejection = %+v, want cause conflict on the index", ej)
	}
}

func TestCompose_ALaterCommitRegeneratesOntoTheRunningTip(t *testing.T) {
	t.Parallel()
	f := newRegenFixture(t)
	peer := f.shardLane(t, 641, "b", "bravo\n")
	f.g.Git("checkout", "-q", "-b", "lane-642", f.base)
	f.g.Write(shardDir+"/d.md", "delta\n")
	f.g.Write(indexFile, aggregate("alpha\n", "delta\n"))
	f.g.Git("add", ".")
	f.g.Git("commit", "-q", "-m", "step 0")
	first := headOf(f.g)
	f.g.Write(shardDir+"/c.md", "charlie\n")
	f.g.Write(indexFile, aggregate("alpha\n", "charlie\n", "delta\n"))
	f.g.Git("add", ".")
	f.g.Git("commit", "-q", "-m", "step 1")
	second := headOf(f.g)
	f.g.Git("checkout", "-q", "main")
	rec := consoleRecord()
	rec.Commits = nil
	for _, sha := range []string{first, second} {
		patch, err := f.repo.PatchID(context.Background(), sha)
		if err != nil {
			t.Fatal(err)
		}
		rec.Commits = append(rec.Commits, CommitIdentity{SHA: sha, BaseSHA: f.base, PatchID: patch})
	}
	rec.Audit = AuditBinding{ArtifactSHA: fixtureArtifact, Class: "docs-review"}
	s := &shardRegen{t: t, repo: f.repo}
	comp := regening(f, s)
	got, err := comp.Compose(context.Background(), f.batch(t), peer)
	if err != nil {
		t.Fatal(err)
	}
	before := got.Batch.Tip
	got, err = comp.Compose(context.Background(), got.Batch, rec)
	if err != nil {
		t.Fatalf("Compose(console PR): %v", err)
	}
	if want := aggregate("alpha\n", "bravo\n", "charlie\n", "delta\n"); f.g.Git("show", got.Batch.Tip+":"+indexFile) != strings.TrimSpace(want) {
		t.Errorf("the aggregate is %q, want the output of both PRs", f.g.Git("show", got.Batch.Tip+":"+indexFile))
	}
	if s.count() != 2 {
		t.Fatalf("the port ran %d times, want once for each commit", s.count())
	}
	if s.calls[0].onto != before {
		t.Errorf("the first commit regenerated onto %s, want the batch tip %s", s.calls[0].onto, before)
	}
	if onto := s.calls[1].onto; onto == before || f.g.Git("rev-parse", onto+"^") != before {
		t.Errorf("the second commit regenerated onto %s, want the commit that apply made for the first commit on %s", onto, before)
	}
}

func TestCompose_APortErrorEjectsWithTheCauseConflictAndNamesTheError(t *testing.T) {
	t.Parallel()
	f := newRegenFixture(t)
	first := f.shardLane(t, 651, "c", "charlie\n")
	second := f.shardLane(t, 652, "b", "bravo\n")
	s := &shardRegen{t: t, repo: f.repo, fail: errors.New("generator did not build")}
	comp := regening(f, s)
	got, err := comp.Compose(context.Background(), f.batch(t), first)
	if err != nil {
		t.Fatal(err)
	}
	_, err = comp.Compose(context.Background(), got.Batch, second)
	ej := ejectionOfErr(t, err)
	if ej.Cause != CauseConflict || !slices.Equal(ej.Paths, []string{indexFile}) || !strings.Contains(ej.Error(), "generator did not build") {
		t.Errorf("ejection = %+v (%v), want cause conflict, the paths and the port error", ej, ej)
	}
}

func TestCompose_BothStagesRegenerate(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{policy.FleetIntegrationShadow, policy.FleetIntegrationEnforce} {
		f := newRegenFixture(t)
		first := f.shardLane(t, 661, "c", "charlie\n")
		second := f.shardLane(t, 662, "b", "bravo\n")
		s := &shardRegen{t: t, repo: f.repo}
		comp := Composer{Repo: f.repo, Stage: stage, Gate: (&gateLog{}).gate, Regen: s.regen}
		got, err := comp.Compose(context.Background(), f.batch(t), first)
		if err != nil {
			t.Fatalf("%s: %v", stage, err)
		}
		if _, err = comp.Compose(context.Background(), got.Batch, second); err != nil {
			t.Errorf("%s: Compose(second) = %v, want no ejection", stage, err)
		}
		if s.count() != 1 {
			t.Errorf("%s: the port ran %d times, want 1", stage, s.count())
		}
	}
}

type firedCatalogs struct {
	plainCatalogs
	conflicted [][]string
}

func (c *firedCatalogs) Fired(_, _, conflicted []string) []string {
	c.conflicted = append(c.conflicted, conflicted)
	return conflicted
}

func TestCompose_ARegeneratedCommitRecordsTheDerivedMergeInTheProof(t *testing.T) {
	t.Parallel()
	f := newRegenFixture(t)
	first := f.shardLane(t, 671, "c", "charlie\n")
	second := f.shardLane(t, 672, "b", "bravo\n")
	s := &shardRegen{t: t, repo: f.repo}
	cat := &firedCatalogs{}
	comp := regening(f, s)
	comp.Overlap = &OverlapFacts{Catalogs: cat}
	got, err := comp.Compose(context.Background(), f.batch(t), first)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cat.conflicted[len(cat.conflicted)-1], nil) {
		t.Errorf("a clean compose named the conflicted paths %v", cat.conflicted)
	}
	got, err = comp.Compose(context.Background(), got.Batch, second)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Proof.Evidence.Derived, []string{indexFile}) || !slices.Contains(got.Proof.Rules, overlap.RuleDerived) {
		t.Errorf("proof = %+v, want the regenerated path as derived evidence", got.Proof)
	}
}

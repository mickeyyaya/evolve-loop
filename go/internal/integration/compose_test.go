package integration

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/evidence"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/overlap"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	fixtureWave     = "135"
	fixtureArtifact = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sharedFile      = "shared.txt"
)

type composeFixture struct {
	g    *gittest.Repo
	repo Repo
	base string
}

func newComposeFixture(t *testing.T) composeFixture {
	t.Helper()
	g := gittest.Fixture(t)
	g.Write(sharedFile, "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n")
	g.Git("add", ".")
	g.Git("commit", "-q", "-m", "base")
	return composeFixture{g: g, repo: Repo{Dir: g.Dir, Git: SysGit()}, base: headOf(g)}
}

func (f composeFixture) lane(t *testing.T, cycle int, commits ...map[string]string) DeliverableRecord {
	t.Helper()
	f.g.Git("checkout", "-q", "-b", "lane-"+strconv.Itoa(cycle), f.base)
	rec := DeliverableRecord{Wave: fixtureWave, Cycle: cycle, Trailer: CycleTrailer(cycle)}
	for i, files := range commits {
		for _, name := range slices.Sorted(mapKeys(files)) {
			f.g.Write(name, files[name])
		}
		f.g.Git("add", ".")
		f.g.Git("commit", "-q", "-m", "lane "+strconv.Itoa(cycle)+" step "+strconv.Itoa(i))
		sha := headOf(f.g)
		patch, err := f.repo.PatchID(context.Background(), sha)
		if err != nil {
			t.Fatal(err)
		}
		rec.Commits = append(rec.Commits, CommitIdentity{SHA: sha, BaseSHA: f.base, PatchID: patch})
	}
	rec.Audit = AuditBinding{TreeSHA: f.g.Git("rev-parse", "HEAD^{tree}"), ArtifactSHA: fixtureArtifact}
	f.g.Git("checkout", "-q", "main")
	return rec
}

func mapKeys(m map[string]string) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

func (f composeFixture) batch(t *testing.T) Batch {
	t.Helper()
	b, err := f.repo.StartBatch(context.Background(), Batch{Wave: fixtureWave, Index: 1, Base: f.base})
	if err != nil {
		t.Fatalf("StartBatch: %v", err)
	}
	return b
}

func (f composeFixture) show(t *testing.T, rev string) string {
	t.Helper()
	return f.g.Git("show", "-s", "--format=%B", rev)
}

type gateLog struct {
	mu    sync.Mutex
	tips  []string
	tiers []overlap.Tier
	fail  func(tip string) error
}

func (l *gateLog) gate(_ context.Context, tip string, tier overlap.Tier) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tips = append(l.tips, tip)
	l.tiers = append(l.tiers, tier)
	if l.fail != nil {
		return l.fail(tip)
	}
	return nil
}

func (l *gateLog) calls() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.tips)
}

func enforcing(f composeFixture, l *gateLog) Composer {
	return Composer{Repo: f.repo, Stage: policy.FleetIntegrationEnforce, Gate: l.gate}
}

func TestStartBatch_CutsTheBranchAtTheBaseWithoutACheckout(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	f.g.Write("untracked.txt", "keep me\n")
	b := f.batch(t)
	want := Batch{Wave: fixtureWave, Index: 1, Base: f.base, Branch: "integration/w135-b1", Tip: f.base}
	if b != want {
		t.Fatalf("StartBatch = %+v, want %+v", b, want)
	}
	if got := f.g.Git("rev-parse", "refs/heads/integration/w135-b1"); got != f.base {
		t.Errorf("the batch branch is at %s, want the base %s", got, f.base)
	}
	if got := f.g.Git("symbolic-ref", "--short", "HEAD"); got != "main" {
		t.Errorf("StartBatch moved the checkout to %q", got)
	}
	if got := f.g.Git("status", "--porcelain"); got != "?? untracked.txt" {
		t.Errorf("StartBatch touched the work tree: %q", got)
	}
}

func TestStartBatch_RefusesAnUnsafeBatch(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	f.batch(t)
	cases := map[string]Batch{
		"an existing branch":       {Wave: fixtureWave, Index: 1, Base: f.base},
		"a short base":             {Wave: fixtureWave, Index: 2, Base: f.base[:12]},
		"an option as the base":    {Wave: fixtureWave, Index: 2, Base: "--output=/tmp/x"},
		"an index below one":       {Wave: fixtureWave, Index: 0, Base: f.base},
		"a wave that leaves a ref": {Wave: "../main", Index: 2, Base: f.base},
		"an empty wave":            {Wave: "", Index: 2, Base: f.base},
	}
	for name, in := range cases {
		if b, err := f.repo.StartBatch(context.Background(), in); err == nil {
			t.Errorf("%s: StartBatch = %+v, want an error", name, b)
		}
	}
	if got := f.g.Git("for-each-ref", "--format=%(refname)", "refs/heads/integration/"); got != "refs/heads/integration/w135-b1" {
		t.Errorf("a refused batch wrote a branch: %q", got)
	}
}

func TestCompose_OneCommitWithTheCycleAndTheAuditBindingTrailers(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	rec := f.lane(t, 501, map[string]string{"a.txt": "a\n"})
	log := &gateLog{}
	var got Composed
	got, err := enforcing(f, log).Compose(context.Background(), f.batch(t), rec)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if parent := f.g.Git("rev-parse", got.Batch.Tip+"^"); parent != f.base {
		t.Errorf("the composed commit has the parent %s, want the batch base %s", parent, f.base)
	}
	if tip := f.g.Git("rev-parse", "refs/heads/"+got.Batch.Branch); tip != got.Batch.Tip {
		t.Errorf("the batch branch is at %s, want the composed tip %s", tip, got.Batch.Tip)
	}
	msg := f.show(t, got.Batch.Tip)
	for _, line := range []string{
		"evolve-cycle 501: integrate",
		CycleTrailer(501),
		evidence.KeyAuditTree + ": " + rec.Audit.TreeSHA,
		evidence.KeyArtifactSHA + ": " + fixtureArtifact,
	} {
		if !strings.Contains(msg, line) {
			t.Errorf("the composed message lacks %q:\n%s", line, msg)
		}
	}
	if parsed := evidence.Parse(msg); parsed.Cycle != 501 || parsed.ArtifactSHA != fixtureArtifact {
		t.Errorf("evidence.Parse(composed message) = %+v", parsed)
	}
	if !HasCycleTrailer(msg, 501) || !f.repo.LandedOnRef(context.Background(), rec, got.Batch.Branch) {
		t.Error("the I0 landed identity does not find the cycle on the batch branch")
	}
	if log.calls() != 1 || log.tips[0] != got.Batch.Tip || log.tiers[0] != got.Proof.Tier {
		t.Errorf("the gate saw %v %v, want one call on %s at %s", log.tips, log.tiers, got.Batch.Tip, got.Proof.Tier)
	}
}

func TestCompose_AppliesEveryCommitOfTheRecordInOrderAsOneCommit(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	rec := f.lane(t, 502, map[string]string{"a.txt": "first\n"}, map[string]string{"a.txt": "second\n", "b.txt": "b\n"})
	got, err := enforcing(f, &gateLog{}).Compose(context.Background(), f.batch(t), rec)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if n := f.g.Git("rev-list", "--count", f.base+".."+got.Batch.Tip); n != "1" {
		t.Errorf("the compose made %s commits, want 1", n)
	}
	if a, b := f.g.Git("show", got.Batch.Tip+":a.txt"), f.g.Git("show", got.Batch.Tip+":b.txt"); a != "second" || b != "b" {
		t.Errorf("the composed tree holds a.txt=%q b.txt=%q, want the last commit's content", a, b)
	}
}

func TestCompose_ATextualConflictEjectsWithNoGateRun(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	a := f.lane(t, 511, map[string]string{sharedFile: "ONE\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"})
	b := f.lane(t, 512, map[string]string{sharedFile: "uno\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"})
	c := enforcing(f, &gateLog{})
	first, err := c.Compose(context.Background(), f.batch(t), a)
	if err != nil {
		t.Fatalf("Compose(A): %v", err)
	}
	log := &gateLog{}
	_, err = enforcing(f, log).Compose(context.Background(), first.Batch, b)
	var ej *Ejection
	if !errors.As(err, &ej) {
		t.Fatalf("Compose(B) = %v, want an *Ejection", err)
	}
	if ej.Cycle != 512 || ej.Cause != CauseConflict || !slices.Equal(ej.Paths, []string{sharedFile}) {
		t.Errorf("ejection = %+v, want cycle 512, cause conflict, paths [%s]", ej, sharedFile)
	}
	if !strings.Contains(ej.Diff, "+ONE") || !strings.Contains(ej.Error(), "512") {
		t.Errorf("the ejection does not carry the diff the deliverable met: %q / %q", ej.Diff, ej.Error())
	}
	if log.calls() != 0 {
		t.Errorf("a textual conflict ran the gate %d times, want 0", log.calls())
	}
	if tip := f.g.Git("rev-parse", "refs/heads/"+first.Batch.Branch); tip != first.Batch.Tip {
		t.Errorf("an ejection moved the batch branch to %s", tip)
	}
}

func TestCompose_ARedGateEjectsAndKeepsTheBranch(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	rec := f.lane(t, 521, map[string]string{"a.txt": "a\n"})
	b := f.batch(t)
	red := errors.New("TestSomething failed")
	_, err := enforcing(f, &gateLog{fail: func(string) error { return red }}).Compose(context.Background(), b, rec)
	var ej *Ejection
	if !errors.As(err, &ej) || ej.Cause != CauseGate || ej.Cycle != 521 || !errors.Is(ej.Err, red) {
		t.Fatalf("Compose = %v, want a gate ejection of cycle 521 that wraps the gate error", err)
	}
	if ej.Diff != "" || !strings.Contains(ej.Error(), red.Error()) {
		t.Errorf("the first deliverable met no change: diff %q, error %q", ej.Diff, ej.Error())
	}
	if tip := f.g.Git("rev-parse", "refs/heads/"+b.Branch); tip != f.base {
		t.Errorf("a red gate moved the batch branch to %s", tip)
	}
}

func TestCompose_AtShadowComposesTextuallyAndCallsNoGate(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	rec := f.lane(t, 531, map[string]string{"a.txt": "a\n"})
	c := Composer{Repo: f.repo, Stage: policy.FleetIntegrationShadow, Gate: func(context.Context, string, overlap.Tier) error {
		t.Error("the gate ran at shadow")
		return nil
	}}
	got, err := c.Compose(context.Background(), f.batch(t), rec)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if got.Batch.Tip == f.base || !HasCycleTrailer(f.show(t, got.Batch.Tip), 531) {
		t.Errorf("shadow did not compose the deliverable: %+v", got)
	}
	noGate := Composer{Repo: f.repo, Stage: policy.FleetIntegrationShadow}
	if _, err := noGate.Compose(context.Background(), got.Batch, f.lane(t, 532, map[string]string{"b.txt": "b\n"})); err != nil {
		t.Errorf("shadow needs no gate: %v", err)
	}
}

type plainCatalogs struct{}

func (plainCatalogs) Bookkeeping(string) bool                        { return false }
func (plainCatalogs) BuildZone(string) bool                          { return false }
func (plainCatalogs) GateZone(string) bool                           { return false }
func (plainCatalogs) DerivedOutput(overlap.Side, string) bool        { return false }
func (plainCatalogs) DataRead(string) bool                           { return false }
func (plainCatalogs) Fired(lane, peer, conflicted []string) []string { return nil }

func TestCompose_TheTierComesFromTheOverlapProof(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	a := f.lane(t, 541, map[string]string{"docs/a.md": "a\n"})
	b := f.lane(t, 542, map[string]string{"docs/b.md": "b\n"})
	c := f.lane(t, 543, map[string]string{sharedFile: "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nTEN\n"})
	d := f.lane(t, 544, map[string]string{sharedFile: "ONE\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"})
	log := &gateLog{}
	comp := enforcing(f, log)
	comp.Overlap = &OverlapFacts{Catalogs: plainCatalogs{}}
	want := []struct {
		rec  DeliverableRecord
		tier overlap.Tier
		rule overlap.Rule
	}{{a, overlap.T1, overlap.RuleEmptyPeer}, {b, overlap.T1, overlap.RuleDisjoint}, {c, overlap.T3, overlap.RuleUnknown}, {d, overlap.T3, overlap.RuleSharedPath}}
	batch := f.batch(t)
	for _, w := range want {
		got, err := comp.Compose(context.Background(), batch, w.rec)
		if err != nil {
			t.Fatalf("Compose(%d): %v", w.rec.Cycle, err)
		}
		if got.Proof.Tier != w.tier || !slices.Contains(got.Proof.Rules, w.rule) {
			t.Errorf("cycle %d: proof %s %v, want %s with %s", w.rec.Cycle, got.Proof.Tier, got.Proof.Rules, w.tier, w.rule)
		}
		batch = got.Batch
	}
	if !slices.Equal(log.tiers, []overlap.Tier{overlap.T1, overlap.T1, overlap.T3, overlap.T3}) {
		t.Errorf("the gate saw the tiers %v", log.tiers)
	}
}

func TestCompose_WithoutTheOverlapFactsTheTierIsConservative(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	rec := f.lane(t, 551, map[string]string{"docs/a.md": "a\n"})
	for i, facts := range []*OverlapFacts{nil, {}} {
		comp := enforcing(f, &gateLog{})
		comp.Overlap = facts
		b, err := f.repo.StartBatch(context.Background(), Batch{Wave: fixtureWave, Index: i + 1, Base: f.base})
		if err != nil {
			t.Fatal(err)
		}
		got, err := comp.Compose(context.Background(), b, rec)
		if err != nil || got.Proof.Tier != overlap.T3 || !slices.Equal(got.Proof.Rules, []overlap.Rule{overlap.RuleUnknown}) {
			t.Errorf("facts %+v: proof %+v err %v, want T3 unknown", facts, got.Proof, err)
		}
	}
}

func refuseGit(t *testing.T) GitFunc {
	return func(_ context.Context, _, _ string, args ...string) (string, error) {
		t.Errorf("git %v ran on a refused input", args)
		return "", errors.New("refused")
	}
}

func TestCompose_RefusesAnUnsafeInputBeforeGit(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	rec := f.lane(t, 561, map[string]string{"a.txt": "a\n"})
	b := f.batch(t)
	with := func(edit func(*DeliverableRecord)) DeliverableRecord {
		r := rec
		r.Commits = slices.Clone(rec.Commits)
		edit(&r)
		return r
	}
	good := Composer{Repo: Repo{Dir: f.g.Dir, Git: refuseGit(t)}, Stage: policy.FleetIntegrationEnforce, Gate: (&gateLog{}).gate}
	recs := map[string]DeliverableRecord{
		"an invalid record":        with(func(r *DeliverableRecord) { r.Commits = nil }),
		"a short commit":           with(func(r *DeliverableRecord) { r.Commits[0].SHA = r.Commits[0].SHA[:12] }),
		"an option as a commit":    with(func(r *DeliverableRecord) { r.Commits[0].SHA = "--output=/tmp/x" }),
		"no audit binding":         with(func(r *DeliverableRecord) { r.Audit = AuditBinding{} }),
		"a trailer in the tree":    with(func(r *DeliverableRecord) { r.Audit.TreeSHA += "\nEvolve-Cycle: 9" }),
		"a short artifact digest":  with(func(r *DeliverableRecord) { r.Audit.ArtifactSHA = "abc" }),
		"a forged cycle trailer":   with(func(r *DeliverableRecord) { r.Trailer = CycleTrailer(9) }),
		"a lane base not an id":    with(func(r *DeliverableRecord) { r.Commits[0].BaseSHA = "main" }),
		"an uppercase artifact id": with(func(r *DeliverableRecord) { r.Audit.ArtifactSHA = strings.ToUpper(fixtureArtifact) }),
	}
	for name, r := range recs {
		if _, err := good.Compose(context.Background(), b, r); err == nil {
			t.Errorf("%s: Compose accepted it", name)
		}
	}
	batches := map[string]Batch{
		"a renamed branch": {Wave: b.Wave, Index: b.Index, Base: b.Base, Tip: b.Tip, Branch: "main"},
		"a short tip":      {Wave: b.Wave, Index: b.Index, Base: b.Base, Tip: b.Tip[:12], Branch: b.Branch},
		"a short base":     {Wave: b.Wave, Index: b.Index, Base: "HEAD", Tip: b.Tip, Branch: b.Branch},
	}
	for name, bb := range batches {
		if _, err := good.Compose(context.Background(), bb, rec); err == nil {
			t.Errorf("%s: Compose accepted it", name)
		}
	}
	composers := map[string]Composer{
		"an unknown stage":         {Repo: good.Repo, Stage: "on", Gate: good.Gate},
		"no stage":                 {Repo: good.Repo, Gate: good.Gate},
		"enforce without any gate": {Repo: good.Repo, Stage: policy.FleetIntegrationEnforce},
	}
	for name, c := range composers {
		if _, err := c.Compose(context.Background(), b, rec); err == nil {
			t.Errorf("%s: Compose accepted it", name)
		}
	}
}

func failOn(verb string) GitFunc {
	real := SysGit()
	return func(ctx context.Context, dir, stdin string, args ...string) (string, error) {
		if args[0] == verb {
			return "", &GitError{Args: args, Code: 128, Err: errors.New(verb + " broke")}
		}
		return real(ctx, dir, stdin, args...)
	}
}

func TestCompose_AGitFailureIsAnErrorNotAnEjection(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	a := f.lane(t, 571, map[string]string{sharedFile: "ONE\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"})
	b := f.lane(t, 572, map[string]string{sharedFile: "uno\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"})
	first, err := enforcing(f, &gateLog{}).Compose(context.Background(), f.batch(t), a)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		verb string
		rec  DeliverableRecord
	}{{"merge-tree", a}, {"commit-tree", a}, {"diff-tree", a}, {"update-ref", a}, {"diff", b}} {
		comp := Composer{Repo: Repo{Dir: f.g.Dir, Git: failOn(tc.verb)}, Stage: policy.FleetIntegrationShadow}
		_, err := comp.Compose(context.Background(), first.Batch, tc.rec)
		var ej *Ejection
		if err == nil || errors.As(err, &ej) {
			t.Errorf("%s fault: Compose = %v, want a plain error", tc.verb, err)
		}
	}
}

func TestCompose_LosesTheRaceWhenTheBranchMoved(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	b := f.batch(t)
	f.g.Git("update-ref", "refs/heads/"+b.Branch, f.lane(t, 581, map[string]string{"x.txt": "x\n"}).Commits[0].SHA)
	_, err := enforcing(f, &gateLog{}).Compose(context.Background(), b, f.lane(t, 582, map[string]string{"a.txt": "a\n"}))
	if err == nil {
		t.Fatal("Compose wrote over a branch that moved since the batch read its tip")
	}
}

func TestSysGit_AFailureCarriesTheExitCodeAndTheOutput(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	_, err := SysGit()(context.Background(), f.g.Dir, "", "merge-base", "--is-ancestor", f.lane(t, 591, map[string]string{"a.txt": "a\n"}).Commits[0].SHA, f.base)
	var ge *GitError
	if !errors.As(err, &ge) || ge.Code != 1 || !slices.Equal(ge.Args[:2], []string{"merge-base", "--is-ancestor"}) {
		t.Fatalf("SysGit error = %#v, want a *GitError with exit 1", err)
	}
	if !strings.Contains(ge.Error(), "exit=1") {
		t.Errorf("GitError.Error() = %q, want the exit code", ge.Error())
	}
	_, err = SysGit()(context.Background(), f.g.Dir, "", "rev-parse", "--verify", "no-such-ref")
	if !errors.As(err, &ge) || ge.Code != 128 || !strings.Contains(ge.Stderr, "no-such-ref") && !strings.Contains(ge.Stderr, "fatal") {
		t.Errorf("SysGit error = %#v, want exit 128 with the stderr", err)
	}
	_, err = SysGit()(context.Background(), "/no/such/dir", "", "status")
	if !errors.As(err, &ge) || ge.Err == nil {
		t.Errorf("SysGit in a missing directory = %#v, want a *GitError that holds the start error", err)
	}
}

func TestCompose_NeverPassesAMergeTreeOutputThatIsNotAnObjectIDToCommitTree(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	rec := f.lane(t, 599, map[string]string{"a.txt": "a\n"})
	b := f.batch(t)
	real := SysGit()
	forged := func(ctx context.Context, dir, stdin string, args ...string) (string, error) {
		switch args[0] {
		case "merge-tree":
			return "--help\n", nil
		case "commit-tree":
			t.Errorf("commit-tree ran with %v", args)
		}
		return real(ctx, dir, stdin, args...)
	}
	c := Composer{Repo: Repo{Dir: f.g.Dir, Git: forged}, Stage: policy.FleetIntegrationShadow}
	if _, err := c.Compose(context.Background(), b, rec); err == nil || !strings.Contains(err.Error(), "full object id") {
		t.Errorf("Compose = %v, want the object id refusal", err)
	}
}

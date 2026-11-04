package integration

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func (f composeFixture) consoleSteps(t *testing.T, pr int, steps ...map[string]string) DeliverableRecord {
	t.Helper()
	f.g.Git("checkout", "-q", "-b", "pr-"+strconv.Itoa(pr), f.base)
	rec := DeliverableRecord{Source: SourceConsolePR, Wave: ConsoleWave, PR: pr, Audit: AuditBinding{TreeSHA: treeSHA40, ArtifactSHA: fixtureArtifact}}
	parent := f.base
	for i, files := range steps {
		for _, name := range slices.Sorted(mapKeys(files)) {
			f.g.Write(name, files[name])
		}
		f.g.Git("add", ".")
		f.g.Git("commit", "-q", "-m", "pr "+strconv.Itoa(pr)+" step "+strconv.Itoa(i))
		sha := headOf(f.g)
		patch, err := f.repo.PatchID(context.Background(), sha)
		if err != nil {
			t.Fatal(err)
		}
		rec.Commits = append(rec.Commits, CommitIdentity{SHA: sha, BaseSHA: parent, PatchID: patch})
		parent = sha
	}
	f.g.Git("checkout", "-q", "main")
	return rec
}

func lineTwo(body string) string {
	return strings.Replace(lineOne, "two", body, 1)
}

func lineFour(body string) string {
	return strings.Replace(lineOne, "four", body, 1)
}

type chainedCompose struct {
	f      composeFixture
	rec    DeliverableRecord
	peer   DeliverableRecord
	batch  Batch
	branch string
}

func composeChained(t *testing.T) chainedCompose {
	t.Helper()
	f := newComposeFixture(t)
	peer := f.consoleSteps(t, 3, map[string]string{sharedFile: lineFour("FOUR")})
	rec := f.consoleSteps(t, 2, map[string]string{sharedFile: lineTwo("TWO")}, map[string]string{sharedFile: lineTwo("Two!")}, map[string]string{"c.txt": "c\n"})
	b := f.batchAt(t, 1)
	c := Composer{Repo: f.repo, Stage: "shadow"}
	first, err := c.Compose(context.Background(), b, peer)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.Compose(context.Background(), first.Batch, rec)
	if err != nil {
		t.Fatal(err)
	}
	return chainedCompose{f: f, rec: rec, peer: peer, batch: second.Batch, branch: branchRef(second.Batch.Branch)}
}

func TestCheckLandedFindsAMultiCommitConsolePRThatComposedBesideAPeer(t *testing.T) {
	t.Parallel()
	c := composeChained(t)
	ctx := context.Background()
	for _, rec := range []DeliverableRecord{c.rec, c.peer} {
		if landed, err := c.f.repo.CheckLanded(ctx, rec, c.branch); err != nil || !landed {
			t.Errorf("%s: CheckLanded = %v, %v, want landed", rec.Label(), landed, err)
		}
	}
	if got := strings.TrimSpace(c.f.g.Git("rev-list", "--count", c.batch.Base+".."+c.batch.Tip)); got != "4" {
		t.Errorf("the batch holds %s commits, want one for the peer and one for each of the three commits", got)
	}
	if landed, err := c.f.repo.CheckLanded(ctx, c.rec, "main"); err != nil || landed {
		t.Errorf("a PR that is not on main read as landed: %v, %v", landed, err)
	}
}

func TestCheckLandedKeepsAnEmptyOwnedCommitFromHidingAPR(t *testing.T) {
	t.Parallel()
	c := composeChained(t)
	c.f.g.Git("checkout", "-q", "-b", "scratch", "main")
	c.f.g.Git("commit", "-q", "--allow-empty", "-m", "evolve-pr 2: integrate\n\nSource-PR: #2")
	if landed, err := c.f.repo.CheckLanded(context.Background(), c.rec, "scratch"); err != nil || landed {
		t.Errorf("CheckLanded = %v, %v, want not landed and no error for an empty commit", landed, err)
	}
}

func TestCheckLandedStopsOnEachGitFailure(t *testing.T) {
	t.Parallel()
	c := composeChained(t)
	sys := SysGit()
	failing := func(match func(args []string) bool) Repo {
		return Repo{Dir: c.f.g.Dir, Git: func(ctx context.Context, dir, stdin string, args ...string) (string, error) {
			if match(args) {
				return "", errors.New("git broke")
			}
			return sys(ctx, dir, stdin, args...)
		}}
	}
	ctx := context.Background()
	cases := map[string]func(args []string) bool{
		"log":           func(a []string) bool { return a[0] == "log" },
		"candidate id":  func(a []string) bool { return a[0] == "diff-tree" && slices.Contains(a, "-U0") },
		"patch of a PR": func(a []string) bool { return a[0] == "diff-tree" && !slices.Contains(a, "-U0") },
	}
	for name, match := range cases {
		if landed, err := failing(match).CheckLanded(ctx, c.rec, c.branch); err == nil || landed {
			t.Errorf("%s: CheckLanded = %v, %v, want an error", name, landed, err)
		}
	}
	uncomposed := c.f.consoleSteps(t, 9, map[string]string{"z.txt": "z\n"})
	if landed, err := failing(cases["candidate id"]).CheckLanded(ctx, uncomposed, c.branch); err == nil || landed {
		t.Errorf("an unreadable commit of the PR: CheckLanded = %v, %v, want an error", landed, err)
	}
}

func TestCheckLandedRefusesARecordWhosePatchIsNotItsCommit(t *testing.T) {
	t.Parallel()
	c := composeChained(t)
	forged := c.rec
	forged.Commits = slices.Clone(c.rec.Commits)
	forged.Commits[0].PatchID = strings.Repeat("0", 40)
	if landed, err := c.f.repo.CheckLanded(context.Background(), forged, c.branch); err != nil || landed {
		t.Errorf("CheckLanded = %v, %v, want not landed and no error", landed, err)
	}
	if landed, err := c.f.repo.CheckLanded(context.Background(), DeliverableRecord{Cycle: 4}, c.branch); err != nil || landed {
		t.Errorf("a record with no commits: %v, %v", landed, err)
	}
}

func TestCheckLandedStopsOnALaneCandidateItCannotRead(t *testing.T) {
	t.Parallel()
	c := composeLane(t, "compose: the lane work\n\n"+CycleTrailer(landedCycle))
	sys := SysGit()
	broken := func(ctx context.Context, dir, stdin string, args ...string) (string, error) {
		if args[0] == "diff-tree" && args[len(args)-1] != c.lane {
			return "", errors.New("diff-tree broke")
		}
		return sys(ctx, dir, stdin, args...)
	}
	if landed, err := (Repo{Dir: c.repo.Dir, Git: broken}).CheckLanded(context.Background(), c.record, "HEAD"); err == nil || landed {
		t.Errorf("CheckLanded = %v, %v, want an error", landed, err)
	}
}

func TestMergerEjectsAPRWhoseHeadMovedBeforeTheFastForwardAndLandsTheRest(t *testing.T) {
	t.Parallel()
	r := newLandRig(sha('0'))
	calls := 0
	r.merger.Heads = func(_ context.Context, rs []DeliverableRecord) ([]*Ejection, error) {
		calls++
		if calls == 1 {
			return []*Ejection{{Cycle: 2, Cause: CauseHead}}, nil
		}
		return nil, nil
	}
	out, err := r.merger.Land(context.Background(), recs(1, 2))
	if err != nil || !slices.Equal(out.Landed, []int{1}) || !slices.Equal(out.Ejected, []int{2}) || !slices.Equal(r.ejected, []int{2}) {
		t.Fatalf("out %+v err %v ejected %v, want 1 landed and 2 ejected", out, err, r.ejected)
	}
	if len(r.main.landed) != 1 || !slices.Equal(r.calls[1], []int{1}) {
		t.Errorf("fast-forwards %d composes %v, want one fast-forward after a recompose without PR 2", len(r.main.landed), r.calls)
	}
}

func TestMergerLandsNothingWhenEveryHeadMoved(t *testing.T) {
	t.Parallel()
	r := newLandRig(sha('0'))
	r.merger.Heads = func(_ context.Context, rs []DeliverableRecord) ([]*Ejection, error) {
		return []*Ejection{{Cycle: 1, Cause: CauseHead}}, nil
	}
	out, err := r.merger.Land(context.Background(), recs(1))
	if err != nil || len(out.Landed) != 0 || len(r.main.landed) != 0 || !slices.Equal(r.ejected, []int{1}) {
		t.Errorf("out %+v err %v ff %d ejected %v, want no landing", out, err, len(r.main.landed), r.ejected)
	}
}

func TestMergerStopsWhenTheHeadCheckFailsOrTheBriefFails(t *testing.T) {
	t.Parallel()
	r := newLandRig(sha('0'))
	boom := errors.New("heads down")
	r.merger.Heads = func(context.Context, []DeliverableRecord) ([]*Ejection, error) { return nil, boom }
	if _, err := r.merger.Land(context.Background(), recs(1)); !errors.Is(err, boom) || len(r.main.landed) != 0 {
		t.Errorf("err = %v ff %d, want the head check error and no landing", err, len(r.main.landed))
	}
	brief := errors.New("comment down")
	r.merger.Heads = func(context.Context, []DeliverableRecord) ([]*Ejection, error) {
		return []*Ejection{{Cycle: 1, Cause: CauseHead}}, nil
	}
	r.merger.Eject = func(context.Context, DeliverableRecord, error) error { return brief }
	if _, err := r.merger.Land(context.Background(), recs(1)); !errors.Is(err, brief) {
		t.Errorf("err = %v, want the brief error", err)
	}
}

func (r *trainRig) advance(n int, file, body string) string {
	r.f.g.Git("checkout", "-q", "pr-"+strconv.Itoa(n))
	r.f.g.Write(file, body)
	r.f.g.Git("add", file)
	r.f.g.Git("commit", "-q", "-m", "fix: late")
	r.f.g.Git("push", "-q", "origin", "pr-"+strconv.Itoa(n)+":refs/pull/"+strconv.Itoa(n)+"/head")
	head := headOf(r.f.g)
	r.f.g.Git("checkout", "-q", "main")
	return head
}

func TestTrainMovedHeadsNamesTheCommitsThatDidNotLand(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	rec := r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	other := r.pr(t, 2, "feat: b", map[string]string{"b.txt": "b\n"})
	ctx := context.Background()
	if moved, err := r.train.movedHeads(ctx, []DeliverableRecord{rec, other}); err != nil || len(moved) != 0 {
		t.Fatalf("movedHeads = %v, %v, want none", moved, err)
	}
	late := r.advance(1, "late.txt", "late\n")
	moved, err := r.train.movedHeads(ctx, []DeliverableRecord{rec, other})
	if err != nil || len(moved) != 1 || moved[0].PR != 1 || moved[0].Cause != CauseHead {
		t.Fatalf("movedHeads = %+v, %v, want PR 1 ejected on a head move", moved, err)
	}
	for _, want := range []string{rec.Commits[0].SHA, late} {
		if !strings.Contains(moved[0].Err.Error(), want) || !strings.Contains(Brief(moved[0]), want) {
			t.Errorf("the brief lacks %s: %v", want, moved[0].Err)
		}
	}
}

func TestTrainHeadChecksFailLoudly(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	rec := r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	ctx := context.Background()
	missing := rec
	missing.PR = 77
	if _, err := r.train.headEjection(ctx, missing, false); err == nil {
		t.Error("headEjection ignored a PR with no fetched head")
	}
	r.advance(1, "late.txt", "late\n")
	r.train.sync(ctx, []int{1})
	broken := rec
	broken.Commits = slices.Clone(rec.Commits)
	broken.Commits[0].BaseSHA = treeSHA40
	if _, err := r.train.headEjection(ctx, broken, false); err == nil {
		t.Error("headEjection ignored a base it cannot read")
	}
	if _, err := r.train.movedHeads(ctx, []DeliverableRecord{missing}); err == nil {
		t.Error("movedHeads ignored a PR with no head on the host")
	}
	if _, err := r.train.movedHeads(ctx, []DeliverableRecord{broken}); err == nil {
		t.Error("movedHeads ignored a base it cannot read")
	}
	r.f.g.Git("remote", "set-url", "origin", r.root+"/missing")
	if _, err := r.train.movedHeads(ctx, []DeliverableRecord{rec}); err == nil {
		t.Error("movedHeads ignored an unreachable origin")
	}
}

func TestTrainPlanStopsWhenTheLandedCheckFails(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	sys := SysGit()
	r.train.Composer.Repo.Git = func(ctx context.Context, dir, stdin string, args ...string) (string, error) {
		if args[0] == "log" && slices.Contains(args, "-z") {
			return "", errors.New("log broke")
		}
		return sys(ctx, dir, stdin, args...)
	}
	if _, err := r.train.Plan(context.Background(), []int{1}); err == nil || !strings.Contains(err.Error(), "check whether PR #1 is on main") {
		t.Fatalf("Plan = %v, want the landed check to stop it", err)
	}
}

type lateMain struct {
	GitMain
	after func()
}

func (m lateMain) FastForward(ctx context.Context, tip string) error {
	err := m.GitMain.FastForward(ctx, tip)
	if err == nil && m.after != nil {
		m.after()
	}
	return err
}

func TestTrainLandKeepsAPRWhoseHeadMovedAfterTheFastForward(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a\n\nCloses-Inbox: item-a", map[string]string{"a.txt": "a\n"})
	plan, err := r.train.Plan(context.Background(), []int{1})
	if err != nil {
		t.Fatal(err)
	}
	var late string
	r.train.Main = lateMain{GitMain: GitMain{Repo: r.f.repo}, after: func() { late = r.advance(1, "late.txt", "late\n") }}

	out, err := r.train.Land(context.Background(), plan)

	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.PR != 1 {
		t.Fatalf("Land = %v, want a refusal that names PR 1", err)
	}
	if _, closed := r.host.closed[1]; closed {
		t.Error("a PR with a moved head must stay open")
	}
	if len(r.host.comments[1]) != 1 || !strings.Contains(r.host.comments[1][0], late) {
		t.Errorf("comments = %v, want the new commit %s", r.host.comments[1], late)
	}
	if !slices.Equal(out.Inbox, []string{"item-a"}) {
		t.Errorf("inbox = %v, want the item of the landed commit", out.Inbox)
	}
	if _, ok := FindPRRecord(r.root, 1); ok {
		t.Error("the landed commits are spent")
	}
}

func TestTrainLandReportsAFailureOfTheHeadCheckAfterTheFastForward(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	plan, err := r.train.Plan(context.Background(), []int{1})
	if err != nil {
		t.Fatal(err)
	}
	r.train.Main = lateMain{GitMain: GitMain{Repo: r.f.repo}, after: func() {
		r.f.g.Git("remote", "set-url", "origin", r.root+"/missing")
	}}
	if _, err := r.train.Land(context.Background(), plan); err == nil || len(r.host.closed) != 0 {
		t.Errorf("Land = %v closed %v, want an error and no close", err, r.host.closed)
	}
}

func TestTrainLandFailsWhenTheHeadOfALandedPRCannotBeRead(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	plan, err := r.train.Plan(context.Background(), []int{1})
	if err != nil {
		t.Fatal(err)
	}
	r.train.Main = lateMain{GitMain: GitMain{Repo: r.f.repo}, after: func() {
		r.origin.Git("update-ref", "-d", "refs/pull/1/head")
	}}
	if _, err := r.train.Land(context.Background(), plan); err == nil || len(r.host.closed) != 0 {
		t.Errorf("Land = %v closed %v, want an error and no close", err, r.host.closed)
	}
}

func TestTrainLandFailsWhenTheNewCommitsOfALandedPRCannotBeListed(t *testing.T) {
	t.Parallel()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	plan, err := r.train.Plan(context.Background(), []int{1})
	if err != nil {
		t.Fatal(err)
	}
	sys := SysGit()
	listing := false
	r.train.Composer.Repo.Git = func(ctx context.Context, dir, stdin string, args ...string) (string, error) {
		if listing && args[0] == "rev-list" {
			return "", errors.New("rev-list broke")
		}
		return sys(ctx, dir, stdin, args...)
	}
	r.train.Main = lateMain{GitMain: GitMain{Repo: r.f.repo}, after: func() {
		r.advance(1, "late.txt", "late\n")
		listing = true
	}}
	if _, err := r.train.Land(context.Background(), plan); err == nil || len(r.host.closed) != 0 {
		t.Errorf("Land = %v closed %v, want an error and no close", err, r.host.closed)
	}
}

package integration

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

type movingHead struct {
	GitMain
	head string
}

func (m movingHead) Head(context.Context) (string, error) { return m.head, nil }

func twoPRRig(t *testing.T) (*trainRig, Plan) {
	t.Helper()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a\n\nCloses-Inbox: item-a", map[string]string{"a.txt": "a\n"})
	r.pr(t, 2, "feat: b", map[string]string{"b.txt": "b\n"})
	plan, err := r.train.Plan(context.Background(), []int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	return r, plan
}

func takeToMain(t *testing.T, r *trainRig, rec DeliverableRecord) {
	t.Helper()
	g := r.f.g
	g.Git("checkout", "-q", "-b", "take-"+strconv.Itoa(rec.PR), refMain)
	g.Git("cherry-pick", "--no-commit", rec.Commits[0].SHA)
	g.Git("commit", "-q", "-m", "feat: taken\n\nSource-PR: #"+strconv.Itoa(rec.PR))
	g.Git("push", "-q", "origin", "HEAD:main")
	g.Git("checkout", "-q", "main")
	g.Git("fetch", "-q", "origin", "+refs/heads/main:"+refMain)
}

func TestTrainComposeLeavesOutAMemberThatMainTookAfterThePlan(t *testing.T) {
	t.Parallel()
	r, plan := twoPRRig(t)
	takeToMain(t, r, plan.Kept[0])
	comp, err := (&batcher{t: r.train}).compose(context.Background(), plan.Kept)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(refsOf(comp.Kept), []int{2}) || !slices.Equal(refsOf(comp.OnMain), []int{1}) || len(comp.Ejected) != 0 {
		t.Errorf("kept %v on main %v ejected %v, want PR 2 kept and PR 1 on main", refsOf(comp.Kept), refsOf(comp.OnMain), comp.Ejected)
	}
	if comp.Batch.Base != r.originMain() {
		t.Errorf("base = %s, want the fetched main %s", comp.Batch.Base, r.originMain())
	}
}

func TestTrainComposeOpensNoBatchWhenMainTookEveryMember(t *testing.T) {
	t.Parallel()
	r, plan := twoPRRig(t)
	takeToMain(t, r, plan.Kept[0])
	takeToMain(t, r, plan.Kept[1])
	comp, err := (&batcher{t: r.train}).compose(context.Background(), plan.Kept)
	if err != nil || len(comp.Kept) != 0 || len(comp.OnMain) != 2 || comp.Batch.Branch != "" {
		t.Errorf("compose = %+v, %v, want both on main and no batch branch", comp, err)
	}
}

func TestTrainComposeUsesTheFetchedBaseWhenMainMovesAfterTheFetch(t *testing.T) {
	t.Parallel()
	r, plan := twoPRRig(t)
	fetched := r.originMain()
	r.train.Main = movingHead{GitMain: GitMain{Repo: r.f.repo}, head: sha('7')}
	comp, err := (&batcher{t: r.train}).compose(context.Background(), plan.Kept)
	if err != nil || comp.Batch.Base != fetched || len(comp.Kept) != 2 {
		t.Errorf("compose = %+v, %v, want a batch on the fetched base %s", comp, err, fetched)
	}
}

func TestTrainComposeReportsABaseItCannotRead(t *testing.T) {
	t.Parallel()
	r, plan := twoPRRig(t)
	real := r.train.Composer.Repo.Git
	r.train.Composer.Repo.Git = func(ctx context.Context, dir, in string, args ...string) (string, error) {
		if len(args) > 1 && args[0] == "rev-parse" && args[len(args)-1] == refMain+"^{commit}" {
			return "", errors.New("rev-parse failed")
		}
		return real(ctx, dir, in, args...)
	}
	if _, err := (&batcher{t: r.train}).compose(context.Background(), plan.Kept); err == nil || !strings.Contains(err.Error(), "read the base") {
		t.Errorf("compose = %v, want an error that names the base", err)
	}
}

func TestTrainComposeReportsALandedCheckItCannotRun(t *testing.T) {
	t.Parallel()
	r, plan := twoPRRig(t)
	real := r.train.Composer.Repo.Git
	r.train.Composer.Repo.Git = func(ctx context.Context, dir, in string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "log" {
			return "", errors.New("log failed")
		}
		return real(ctx, dir, in, args...)
	}
	if _, err := (&batcher{t: r.train}).compose(context.Background(), plan.Kept); err == nil || !strings.Contains(err.Error(), "is on main") {
		t.Errorf("compose = %v, want an error from the landed check", err)
	}
}

func TestTrainLandClosesAMemberThatMainTookDuringTheRunAsAlreadyLanded(t *testing.T) {
	t.Parallel()
	r, plan := twoPRRig(t)
	takeToMain(t, r, plan.Kept[0])
	out, err := r.train.Land(context.Background(), plan)
	if err != nil {
		t.Fatalf("Land: %v", err)
	}
	if !slices.Equal(out.Landed, []int{2}) || !slices.Equal(out.OnMain, []int{1}) {
		t.Errorf("landed %v on main %v, want PR 2 landed and PR 1 on main", out.Landed, out.OnMain)
	}
	if body := r.host.closed[1]; body != alreadyLanded {
		t.Errorf("PR 1 close comment = %q, want %q", body, alreadyLanded)
	}
	if _, ok := FindPRRecord(r.root, 1); ok {
		t.Error("the record of a PR that main took must be spent")
	}
	if !slices.Contains(r.inbox, "item-a") {
		t.Errorf("inbox ids = %v, want item-a closed", r.inbox)
	}
	if !strings.Contains(r.host.closed[2], "#1") {
		t.Errorf("PR 2 close comment = %q, want a link to its batch", r.host.closed[2])
	}
}

func TestMergerMovesComposedOnMainMembersToTheOutcomeAndLandsTheRest(t *testing.T) {
	t.Parallel()
	r := newLandRig(sha('0'))
	inner := r.merger.Compose
	r.merger.Compose = func(ctx context.Context, rs []DeliverableRecord) (Composition, error) {
		comp, err := inner(ctx, rs[1:])
		comp.OnMain = rs[:1]
		return comp, err
	}
	out, err := r.merger.Land(context.Background(), recs(1, 2))
	if err != nil || !slices.Equal(out.OnMain, []int{1}) || !slices.Equal(out.Landed, []int{2}) {
		t.Errorf("Land = %+v, %v, want 1 on main and 2 landed", out, err)
	}
}

func TestMergerOpensNoBatchWhenEveryMemberIsOnMain(t *testing.T) {
	t.Parallel()
	r := newLandRig(sha('0'))
	r.merger.Compose = func(_ context.Context, rs []DeliverableRecord) (Composition, error) {
		return Composition{OnMain: rs}, nil
	}
	out, err := r.merger.Land(context.Background(), recs(1, 2))
	if err != nil || !slices.Equal(out.OnMain, []int{1, 2}) || len(r.ci.opened) != 0 || len(out.Landed) != 0 {
		t.Errorf("Land = %+v, %v opened %d, want both on main and no batch", out, err, len(r.ci.opened))
	}
}

func TestEnforceActorMarksAMemberThatMainTookAsMerged(t *testing.T) {
	t.Parallel()
	ports := &stubEnforcePorts{}
	a := EnforceActor{
		Merger: Merger{
			CI:   stubCI{verdict: Green},
			Main: &stubMain{head: testSHA},
			Compose: func(_ context.Context, recs []DeliverableRecord) (Composition, error) {
				return Composition{OnMain: recs}, nil
			},
		},
		Promote: ports,
		Now:     func() time.Time { return time.Unix(100, 0).UTC() },
	}
	res, err := a.Flush(context.Background(), PendingBatch{Pool: 1, Index: 1, Base: testSHA, Members: []DeliverableRecord{{Cycle: 9}}})
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got := res.Members[9]; got.Status != MemberMerged || !got.Landed {
		t.Errorf("member = %+v, want merged and landed", got)
	}
}

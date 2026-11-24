package integration

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/overlap"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type scriptCI struct {
	verdicts []Verdict
	opened   int
}

func (c *scriptCI) Open(_ context.Context, b Batch) (PR, error) {
	c.opened++
	return PR{Number: 100 + c.opened, Tip: b.Tip}, nil
}

func (c *scriptCI) Watch(context.Context, PR) (Verdict, error) {
	v := c.verdicts[0]
	c.verdicts = c.verdicts[1:]
	return v, nil
}

func (c *scriptCI) Rerun(context.Context, PR) error { return nil }

type laneWorld struct {
	origin *gittest.Repo
	plane  *gittest.Repo
	repo   Repo
	base   string
	art    int
}

func newLaneWorld(t *testing.T) *laneWorld {
	t.Helper()
	seed := gittest.Fixture(t)
	for _, f := range []string{"a", "b", "c", "d"} {
		seed.Write(f+".txt", "base\n")
	}
	seed.Git("add", "-A")
	seed.Git("commit", "-q", "-m", "seed")
	origin := gittest.Bare(t)
	seed.Git("push", "-q", origin.Dir, "main")
	plane := gittest.Clone(t, origin.Dir)
	return &laneWorld{origin: origin, plane: plane, repo: Repo{Dir: plane.Dir, Git: SysGit()}, base: seed.Git("rev-parse", "HEAD")}
}

func (w *laneWorld) deliver(t *testing.T, cycle int, file string) DeliverableRecord {
	t.Helper()
	lane := gittest.Clone(t, w.origin.Dir)
	lane.Write(file, "from cycle\n"+file+"\n")
	lane.Git("add", "-A")
	lane.Git("commit", "-q", "-m", "lane "+file)
	sha := lane.Git("rev-parse", "HEAD")
	w.plane.Git("fetch", "-q", lane.Dir, sha)
	patch, err := w.repo.PatchID(context.Background(), sha)
	if err != nil {
		t.Fatal(err)
	}
	return DeliverableRecord{
		Wave: "1", Cycle: cycle, Trailer: CycleTrailer(cycle),
		Commits:     []CommitIdentity{{SHA: sha, BaseSHA: w.base, PatchID: patch}},
		Audit:       AuditBinding{TreeSHA: strings.Repeat("1", 40), ArtifactSHA: strings.Repeat("a", 64)},
		Scope:       []string{file},
		Risk:        "low",
		DeliveredAt: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC),
	}
}

func (w *laneWorld) actor(ci CI, ports *stubEnforcePorts, sink Publisher) EnforceActor {
	return EnforceActor{
		Repo:    w.repo,
		Merger:  Merger{CI: ci, Main: GitMain{Repo: w.repo}},
		Batcher: LaneBatcher{Repo: w.repo, Composer: Composer{Repo: w.repo, Gate: func(context.Context, string, overlap.Tier) error { return nil }}, Wave: "p1"},
		Plane:   PlaneOf(w.repo),
		Requeue: ports,
		Briefs:  ports,
		Promote: ports,
		Publish: sink,
	}
}

func (w *laneWorld) onOrigin(file string) bool {
	return w.origin.Git("show", "main:"+file) != "base"
}

type eventSink struct{ events []signalcenter.Event }

func (s *eventSink) Emit(e signalcenter.Event) { s.events = append(s.events, e) }

func TestLaneBatcher_AReCompositionAndASecondFlushCutFromTheLiveOriginMain(t *testing.T) {
	w := newLaneWorld(t)
	recA, recB, recC := w.deliver(t, 11, "a.txt"), w.deliver(t, 12, "b.txt"), w.deliver(t, 13, "c.txt")
	recD := w.deliver(t, 14, "d.txt")
	ports := &stubEnforcePorts{syncOut: SyncCurrent}
	ci := &scriptCI{verdicts: []Verdict{Red, Red, Green, Green, Green}}
	actor := w.actor(ci, ports, nil)

	first, err := actor.Flush(context.Background(), PendingBatch{Pool: 1, Index: 1, Base: w.base, Members: []DeliverableRecord{recA, recB, recC}})
	if err != nil {
		t.Fatalf("first Flush: %v", err)
	}
	afterFirst := w.origin.Git("rev-parse", "main")
	for _, c := range []int{11, 12, 13} {
		if first.Members[c].Status != MemberMerged || !first.Members[c].Landed {
			t.Fatalf("member %d = %+v, want merged and landed", c, first.Members[c])
		}
	}
	if first.Batch.Branch != PoolBranch(1, 1) || first.Batch.Base != w.origin.Git("rev-parse", "main~2") {
		t.Fatalf("first batch = %+v, want branch %s and the base that its last cut used (main~2)", first.Batch, PoolBranch(1, 1))
	}

	second, err := actor.Flush(context.Background(), PendingBatch{Pool: 1, Index: 2, Base: w.base, Members: []DeliverableRecord{recD}})
	if err != nil {
		t.Fatalf("second Flush: %v", err)
	}
	if second.Batch.Branch != PoolBranch(1, 2) || second.Batch.Base != afterFirst {
		t.Fatalf("second batch = %+v, want branch %s cut at the live origin main %s", second.Batch, PoolBranch(1, 2), afterFirst)
	}
	if !w.onOrigin("d.txt") || second.Members[14].Status != MemberMerged {
		t.Fatalf("member 14 = %+v, d.txt on origin = %v", second.Members[14], w.onOrigin("d.txt"))
	}
	rec := WaveRecord{Pool: 1, Epoch: 1, ShadowBase: w.base, Batches: []IntegrationBatch{first.Batch, second.Batch}, Members: map[int]MemberState{}}
	for _, c := range []int{11, 12, 13, 14} {
		rec.Members[c] = MemberState{Status: MemberMerged}
	}
	if err := rec.Validate(); err != nil {
		t.Fatalf("the wave record of two batches does not validate: %v", err)
	}
	if got, want := w.plane.Git("rev-parse", "main"), w.origin.Git("rev-parse", "main"); got != want {
		t.Fatalf("plane main %s != origin main %s after a flush with no WaveEnd", got, want)
	}
}

func TestEnforceActor_FlushEmitsMergedPerLandedMember(t *testing.T) {
	w := newLaneWorld(t)
	recA := w.deliver(t, 11, "a.txt")
	ports := &stubEnforcePorts{syncOut: SyncCurrent}
	sink := &eventSink{}
	merged := w.actor(&scriptCI{verdicts: []Verdict{Green}}, ports, sink)

	res, err := merged.Flush(context.Background(), PendingBatch{Pool: 1, Index: 1, Base: w.base, Members: []DeliverableRecord{recA}})
	if err != nil || res.Members[11].Status != MemberMerged {
		t.Fatalf("Flush = %+v, %v", res, err)
	}
	var got []signalcenter.Event
	for _, e := range sink.events {
		if e.Code == CodeMerged {
			got = append(got, e)
		}
	}
	if len(got) != 1 || got[0].Kind != signalcenter.KindShipMerged || got[0].Cycle != 11 || got[0].Fields["cycle"] != "11" || got[0].Fields["pr"] != "101" || got[0].Fields["tip"] != w.origin.Git("rev-parse", "main") {
		t.Fatalf("merged events = %+v, want one ship.merged for cycle 11 with the PR and the tip", got)
	}
}

type stepPlane struct {
	out SyncOutcome
	err error
	n   int
}

func (p *stepPlane) Sync(context.Context) (SyncOutcome, error) { p.n++; return p.out, p.err }

func TestEnforceActor_APlaneThatDidNotMoveIsAFailureAndKeepsTheMerge(t *testing.T) {
	for name, plane := range map[string]*stepPlane{
		"blocked":  {out: SyncBlocked},
		"diverged": {out: SyncDiverged},
		"error":    {err: errors.New("fetch boom")},
	} {
		t.Run(name, func(t *testing.T) {
			w := newLaneWorld(t)
			rec := w.deliver(t, 11, "a.txt")
			actor := w.actor(&scriptCI{verdicts: []Verdict{Green}}, &stubEnforcePorts{}, nil)
			actor.Plane = plane

			res, err := actor.Flush(context.Background(), PendingBatch{Pool: 1, Index: 1, Base: w.base, Members: []DeliverableRecord{rec}})

			if err != nil {
				t.Fatalf("Flush = %v; a plane that did not move must not fail the flush", err)
			}
			if res.Members[11].Status != MemberMerged || !w.onOrigin("a.txt") {
				t.Fatalf("member = %+v", res.Members[11])
			}
			if len(res.Failures) != 1 || res.Failures[0].Step != "plane sync" || plane.n != 1 {
				t.Fatalf("failures = %+v, syncs = %d, want one plane sync failure", res.Failures, plane.n)
			}
		})
	}
}

func TestEnforceActor_AFlushWithNoMergeDoesNotSyncThePlane(t *testing.T) {
	w := newLaneWorld(t)
	rec := w.deliver(t, 11, "a.txt")
	plane := &stepPlane{out: SyncCurrent}
	actor := w.actor(&scriptCI{verdicts: []Verdict{Red, Red}}, &stubEnforcePorts{briefPath: "/tmp/b"}, nil)
	actor.Plane = plane

	res, _ := actor.Flush(context.Background(), PendingBatch{Pool: 1, Index: 1, Base: w.base, Members: []DeliverableRecord{rec}})

	if res.Members[11].Status == MemberMerged || plane.n != 0 {
		t.Fatalf("member = %+v, plane syncs = %d, want no merge and no sync", res.Members[11], plane.n)
	}
}

func TestEnforceActor_AMergeFollowedByAnErrorIsStillInTheResult(t *testing.T) {
	t.Run("the second half of a split fails after the first landed", func(t *testing.T) {
		w := newLaneWorld(t)
		recs := []DeliverableRecord{w.deliver(t, 11, "a.txt"), w.deliver(t, 12, "b.txt")}
		ci := &errAfterCI{scriptCI: scriptCI{verdicts: []Verdict{Red, Red, Green}}, failOpenAt: 3}
		actor := w.actor(ci, &stubEnforcePorts{}, nil)

		res, err := actor.Flush(context.Background(), PendingBatch{Pool: 1, Index: 1, Base: w.base, Members: recs})

		if err == nil {
			t.Fatal("Flush = nil, want the error of the second half")
		}
		if res.Members[11].Status != MemberMerged || !w.onOrigin("a.txt") || w.onOrigin("b.txt") {
			t.Fatalf("member 11 = %+v, want merged because it is on origin main", res.Members[11])
		}
		if res.Members[12].Status == MemberMerged {
			t.Fatalf("member 12 = %+v, want not merged", res.Members[12])
		}
	})
	t.Run("a promote error after a merge", func(t *testing.T) {
		w := newLaneWorld(t)
		rec := w.deliver(t, 11, "a.txt")
		actor := w.actor(&scriptCI{verdicts: []Verdict{Green}}, &stubEnforcePorts{}, nil)
		actor.Promote = failingPort{promoteErr: errors.New("promote boom")}

		res, err := actor.Flush(context.Background(), PendingBatch{Pool: 1, Index: 1, Base: w.base, Members: []DeliverableRecord{rec}})

		if err == nil || res.Members[11].Status != MemberMerged || res.Batch.Branch != PoolBranch(1, 1) {
			t.Fatalf("res = %+v, err = %v; want the merge kept in the result and the error returned", res, err)
		}
	})
}

type errAfterCI struct {
	scriptCI
	failOpenAt int
}

func (c *errAfterCI) Open(ctx context.Context, b Batch) (PR, error) {
	pr, err := c.scriptCI.Open(ctx, b)
	if c.opened >= c.failOpenAt {
		return PR{}, errors.New("open boom")
	}
	return pr, err
}

func TestEnforceActor_AnEjectionWithNoRequeuePortIsALoudFailure(t *testing.T) {
	w := newLaneWorld(t)
	rec := w.deliver(t, 11, "a.txt")
	var res FlushResult
	actor := w.actor(&scriptCI{}, &stubEnforcePorts{briefPath: "/tmp/b"}, nil)
	actor.Requeue = nil

	err := actor.ejectMember(context.Background(), rec, errors.New("boom"), &res)

	if !errors.Is(err, errNoRequeue) || len(res.Failures) != 1 || res.Failures[0].Step != "requeue" {
		t.Fatalf("err = %v, failures = %+v", err, res.Failures)
	}
}

func TestEnforceActor_TwoComposeHomesRefuseTheFlush(t *testing.T) {
	w := newLaneWorld(t)
	actor := w.actor(&scriptCI{}, &stubEnforcePorts{}, nil)
	actor.Merger.Compose = func(context.Context, []DeliverableRecord) (Composition, error) { return Composition{}, nil }

	_, err := actor.Flush(context.Background(), PendingBatch{Pool: 1, Index: 1, Base: w.base, Members: []DeliverableRecord{{Cycle: 1}}})

	if !errors.Is(err, errTwoComposeHomes) {
		t.Fatalf("Flush = %v, want %v", err, errTwoComposeHomes)
	}
}

func TestEnforceActor_MissingNamesEveryPortThatEnforceNeeds(t *testing.T) {
	gate := func(context.Context, string, overlap.Tier) error { return nil }
	full := EnforceActor{
		Merger:  Merger{CI: &scriptCI{}, Main: GitMain{}},
		Batcher: LaneBatcher{Composer: Composer{Gate: gate}},
		Plane:   &stepPlane{}, Requeue: &stubEnforcePorts{}, Briefs: &stubEnforcePorts{}, Promote: &stubEnforcePorts{},
	}
	if got := full.Missing(); len(got) != 0 {
		t.Fatalf("Missing of a full actor = %v, want none", got)
	}
	cases := map[string]func(EnforceActor) EnforceActor{
		"Requeue": func(a EnforceActor) EnforceActor { a.Requeue = nil; return a },
		"Promote": func(a EnforceActor) EnforceActor { a.Promote = nil; return a },
		"Briefs":  func(a EnforceActor) EnforceActor { a.Briefs = nil; return a },
		"Plane":   func(a EnforceActor) EnforceActor { a.Plane = nil; return a },
		"CI":      func(a EnforceActor) EnforceActor { a.Merger.CI = nil; return a },
		"Main":    func(a EnforceActor) EnforceActor { a.Merger.Main = nil; return a },
		"Batcher": func(a EnforceActor) EnforceActor { a.Batcher = nil; return a },
		"Gate":    func(a EnforceActor) EnforceActor { a.Batcher = LaneBatcher{}; return a },
	}
	for want, strip := range cases {
		if got := strip(full).Missing(); !slices.Equal(got, []string{want}) {
			t.Errorf("without %s: Missing = %v, want [%s]", want, got, want)
		}
	}
}

func TestLaneBatcher_ComposesAtTheEnforceStageEvenFromAShadowComposer(t *testing.T) {
	w := newLaneWorld(t)
	rec := w.deliver(t, 11, "a.txt")
	gated := 0
	composer := Composer{Repo: w.repo, Stage: policy.FleetIntegrationShadow, Gate: func(context.Context, string, overlap.Tier) error { gated++; return nil }}
	compose := LaneBatcher{Repo: w.repo, Composer: composer, Wave: "p1"}.For(PendingBatch{Pool: 1, Index: 1})

	if _, err := compose(context.Background(), []DeliverableRecord{rec}); err != nil || gated != 1 {
		t.Fatalf("compose err = %v, gate calls = %d, want the gate to run once", err, gated)
	}
}

func TestLaneBatcher_AFailedFetchOrCutIsAnError(t *testing.T) {
	w := newLaneWorld(t)
	boom := errors.New("boom")
	failing := func(fail string) Repo {
		return Repo{Dir: w.plane.Dir, Git: func(ctx context.Context, dir, in string, args ...string) (string, error) {
			if slices.Contains(args, fail) {
				return "", boom
			}
			return SysGit()(ctx, dir, in, args...)
		}}
	}
	for _, step := range []string{"update-ref", "fetch", "rev-parse"} {
		b := LaneBatcher{Repo: failing(step), Composer: Composer{Repo: w.repo}, Wave: "p1"}
		if _, err := b.For(PendingBatch{Pool: 1, Index: 1})(context.Background(), nil); !errors.Is(err, boom) {
			t.Errorf("%s failure: err = %v", step, err)
		}
	}
}

func TestLaneBatcher_AComposeErrorThatIsNotAnEjectionStopsTheComposition(t *testing.T) {
	w := newLaneWorld(t)
	bad := DeliverableRecord{Cycle: 11}
	_, err := LaneBatcher{Repo: w.repo, Composer: Composer{Repo: w.repo}, Wave: "p1"}.For(PendingBatch{Pool: 1, Index: 1})(context.Background(), []DeliverableRecord{bad})
	if err == nil {
		t.Fatal("compose of an invalid record = nil")
	}
}

func TestPlaneOf_SyncsThePlaneToOrigin(t *testing.T) {
	w := newLaneWorld(t)
	other := gittest.Clone(t, w.origin.Dir)
	other.Write("a.txt", "moved\n")
	other.Git("add", "-A")
	other.Git("commit", "-q", "-m", "console")
	other.Git("push", "-q", "origin", "main")

	out, err := PlaneOf(w.repo).Sync(context.Background())

	if err != nil || out != SyncMoved || w.plane.Git("rev-parse", "main") != w.origin.Git("rev-parse", "main") {
		t.Fatalf("Sync = %v, %v; want moved and equal mains", out, err)
	}
}

func TestIsShadow_ReadsTheShadowFieldOfAnEvent(t *testing.T) {
	if !IsShadow(map[string]string{fieldShadow: shadowTrue}) || IsShadow(map[string]string{}) || IsShadow(map[string]string{fieldShadow: "false"}) {
		t.Fatal("IsShadow does not follow the shadow field")
	}
}

func TestLaneBatcher_AConflictingMemberIsEjectedAndTheRestStayKept(t *testing.T) {
	w := newLaneWorld(t)
	first := w.deliver(t, 11, "a.txt")
	clash := w.deliver(t, 12, "a.txt")
	w.plane.Git("checkout", "-q", "--detach", w.base)
	w.plane.Write("a.txt", "other text\n")
	w.plane.Git("add", "-A")
	w.plane.Git("commit", "-q", "-m", "other")
	clash.Commits[0].SHA = w.plane.Git("rev-parse", "HEAD")
	w.plane.Git("checkout", "-q", "main")
	composer := Composer{Repo: w.repo, Gate: func(context.Context, string, overlap.Tier) error { return nil }}

	comp, err := LaneBatcher{Repo: w.repo, Composer: composer, Wave: "p1"}.For(PendingBatch{Pool: 1, Index: 1})(context.Background(), []DeliverableRecord{first, clash})

	if err != nil || len(comp.Kept) != 1 || comp.Kept[0].Cycle != 11 || len(comp.Ejected) != 1 || comp.Ejected[0].Cycle != 12 || comp.Ejected[0].Cause != CauseConflict {
		t.Fatalf("composition = %+v, err = %v; want cycle 11 kept and cycle 12 ejected on a conflict", comp, err)
	}
}

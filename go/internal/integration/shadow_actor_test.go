package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func shadowActorFor(f composeFixture, pub Publisher) ShadowActor {
	f.g.Git("update-ref", PrivateMainRef, f.base)
	return ShadowActor{Composer: ShadowComposer(f.repo), Publish: pub, Now: func() time.Time { return driverNow }}
}

func pendingOf(f composeFixture, pool int, members ...DeliverableRecord) PendingBatch {
	return PendingBatch{Pool: pool, Index: 1, Base: f.base, Trigger: TriggerWaveEnd, Members: members}
}

func TestShadowActor_FlushComposesEachMemberOnThePoolBranchAndPublishesComposed(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	a := f.lane(t, 6, map[string]string{"a.txt": "a\n"})
	b := f.lane(t, 7, map[string]string{"b.txt": "b\n"})
	pub := &sink{}

	res, err := shadowActorFor(f, pub).Flush(context.Background(), pendingOf(f, 9, a, b))

	if err != nil {
		t.Fatal(err)
	}
	if res.Batch.Branch != "integration/p9-b1" || res.Batch.Base != f.base || res.Batch.Tip == f.base || len(res.Batch.Members) != 2 {
		t.Fatalf("batch = %+v", res.Batch)
	}
	if got := f.g.Git("rev-parse", "refs/heads/integration/p9-b1"); got != res.Batch.Tip {
		t.Errorf("branch at %s, want the tip %s", got, res.Batch.Tip)
	}
	for _, c := range []int{6, 7} {
		if res.Members[c].Status != MemberComposed {
			t.Errorf("member %d = %+v", c, res.Members[c])
		}
	}
	if got := pub.codes(); len(got) != 2 || got[0] != "SHIP_INTEGRATION_COMPOSED" {
		t.Fatalf("signals = %v", got)
	}
	e := pub.events[0]
	if e.Fields["shadow"] != "true" || e.Fields["batch"] != "integration/p9-b1" || e.Fields["cycle"] != "6" || e.Kind != signalcenter.KindShipDelivered || e.Severity != signalcenter.SeverityInfo {
		t.Errorf("composed event = %+v", e)
	}
	if e.Reason != "shadow: cycle 6 composed onto integration/p9-b1" {
		t.Errorf("reason = %q", e.Reason)
	}
	if got := f.g.Git("remote"); got != "" {
		t.Errorf("remotes = %q, the shadow compose needs none", got)
	}
}

func TestShadowActor_AConflictIsCountedNotBriefedAndTheChainGoesOn(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	a := f.lane(t, 6, map[string]string{sharedFile: "ONE\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"})
	b := f.lane(t, 7, map[string]string{sharedFile: "uno\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"})
	c := f.lane(t, 8, map[string]string{"c.txt": "c\n"})
	pub := &sink{}

	res, err := shadowActorFor(f, pub).Flush(context.Background(), pendingOf(f, 9, a, b, c))

	if err != nil {
		t.Fatal(err)
	}
	if res.Members[6].Status != MemberComposed || res.Members[7].Status != MemberConflicted || res.Members[8].Status != MemberComposed {
		t.Fatalf("members = %+v", res.Members)
	}
	var ejected *signalcenter.Event
	for i := range pub.events {
		if pub.events[i].Code == CodeEjected {
			ejected = &pub.events[i]
		}
	}
	if ejected == nil {
		t.Fatalf("signals = %v, want an EJECTED", pub.codes())
	}
	if ejected.Fields["shadow"] != "true" || ejected.Fields[FieldCause] != CauseConflict || ejected.Fields[FieldPaths] != sharedFile || ejected.Fields[FieldBriefFile] != "" || ejected.Severity != signalcenter.SeverityWarn {
		t.Errorf("ejected event = %+v", ejected)
	}
	if !strings.Contains(ejected.Reason, "no brief is written and no lane is told") {
		t.Errorf("reason = %q", ejected.Reason)
	}
}

func TestShadowActor_ALaneForkedAfterTheHeldBaseReadsConflictedBaseAhead(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	first := f.lane(t, 6, map[string]string{sharedFile: "ONE\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"})
	f.g.Write(sharedFile, "uno\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n")
	f.g.Git("add", ".")
	f.g.Git("commit", "-q", "-m", "console moved main")
	ahead := headOf(f.g)
	f.g.Write(sharedFile, "UNO\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n")
	f.g.Git("add", ".")
	f.g.Git("commit", "-q", "-m", "lane work on the moved main")
	second := DeliverableRecord{Wave: fixtureWave, Cycle: 7, Trailer: CycleTrailer(7)}
	patch, err := f.repo.PatchID(context.Background(), headOf(f.g))
	if err != nil {
		t.Fatal(err)
	}
	second.Commits = []CommitIdentity{{SHA: headOf(f.g), BaseSHA: ahead, PatchID: patch}}
	second.Audit = AuditBinding{TreeSHA: f.g.Git("rev-parse", "HEAD^{tree}"), ArtifactSHA: fixtureArtifact}

	res, err := shadowActorFor(f, nil).Flush(context.Background(), pendingOf(f, 9, first, second))

	if err != nil {
		t.Fatal(err)
	}
	if res.Members[7].Status != MemberConflictedBaseAhead {
		t.Fatalf("member 7 = %+v, want conflicted_base_ahead", res.Members[7])
	}
}

func TestShadowActor_LandedIsReadFromTheAncestryOfThePrivateRefNotFromACompose(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	landed := f.lane(t, 6, map[string]string{"a.txt": "a\n"})
	other := f.lane(t, 7, map[string]string{"b.txt": "b\n"})
	a := shadowActorFor(f, nil)
	f.g.Git("update-ref", PrivateMainRef, landed.Commits[0].SHA)

	res, err := a.Flush(context.Background(), pendingOf(f, 9, landed, other))

	if err != nil {
		t.Fatal(err)
	}
	if !res.Members[6].Landed || res.Members[7].Landed {
		t.Fatalf("members = %+v, want only cycle 6 landed", res.Members)
	}
	if res.Members[6].Status != MemberComposed {
		t.Errorf("member 6 = %+v, a landed member is still composed", res.Members[6])
	}
}

func TestShadowActor_AMissingPrivateRefIsAFailureAndNotLanded(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	a := f.lane(t, 6, map[string]string{"a.txt": "a\n"})
	actor := ShadowActor{Composer: ShadowComposer(f.repo), Now: func() time.Time { return driverNow }}

	res, err := actor.Flush(context.Background(), pendingOf(f, 9, a))

	if err != nil || res.Members[6].Landed {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if len(res.Failures) != 1 || res.Failures[0].Step != "landed" {
		t.Fatalf("failures = %+v, want one landed failure", res.Failures)
	}
}

func TestShadowActor_ARefreshFailureIsRecordedAndIsNotFatal(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	a := f.lane(t, 6, map[string]string{"a.txt": "a\n"})
	actor := shadowActorFor(f, nil)
	actor.Refresh = func(context.Context) error { return errors.New("offline") }

	res, err := actor.Flush(context.Background(), pendingOf(f, 9, a))

	if err != nil || res.Members[6].Status != MemberComposed {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if len(res.Failures) != 1 || res.Failures[0].Step != "refresh" || res.Failures[0].Error != "offline" {
		t.Fatalf("failures = %+v", res.Failures)
	}
}

func TestShadowActor_AComposeThatCannotRunMarksTheMemberFailed(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	a := f.lane(t, 6, map[string]string{"a.txt": "a\n"})
	a.Trailer = "wrong"

	res, err := shadowActorFor(f, nil).Flush(context.Background(), pendingOf(f, 9, a))

	if err != nil || res.Members[6].Status != MemberFailed {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if len(res.Failures) != 1 || res.Failures[0].Step != "compose" {
		t.Fatalf("failures = %+v", res.Failures)
	}
}

func TestShadowActor_AFlushThatCannotCutItsBatchFailsTheFlush(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	p := pendingOf(f, 9)
	p.Base = "short"
	if _, err := shadowActorFor(f, nil).Flush(context.Background(), p); err == nil {
		t.Fatal("a bad base was accepted")
	}
}

func TestShadowActor_ResumeDeletesOnlyTheStaleRefsOfItsOwnPool(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	for _, name := range []string{"integration/p9-b1", "integration/p9-b2", "integration/p90-b1", "integration/p8-b1", "integration/w9-b1"} {
		f.g.Git("update-ref", "refs/heads/"+name, f.base)
	}

	rec, err := shadowActorFor(f, nil).Resume(context.Background(), WaveRecord{Pool: 9})

	if err != nil || rec.Pool != 9 {
		t.Fatalf("Resume = %+v, %v", rec, err)
	}
	left := f.g.Git("for-each-ref", "--format=%(refname:short)", "refs/heads/integration")
	if left != "integration/p8-b1\nintegration/p90-b1\nintegration/w9-b1" {
		t.Fatalf("refs left:\n%s", left)
	}
}

func TestShadowActor_ResumeReportsAnUnreadableRefStore(t *testing.T) {
	t.Parallel()
	a := ShadowActor{Composer: ShadowComposer(Repo{Dir: t.TempDir(), Git: func(context.Context, string, string, ...string) (string, error) { return "", errors.New("no git") }})}
	if _, err := a.Resume(context.Background(), WaveRecord{Pool: 9}); err == nil {
		t.Fatal("Resume hid a git failure")
	}
}

func TestShadowActor_WaveEndDeletesTheBatchRefsAndRereadsLandedForEachMember(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	a := f.lane(t, 6, map[string]string{"a.txt": "a\n"})
	b := f.lane(t, 7, map[string]string{"b.txt": "b\n"})
	actor := shadowActorFor(f, nil)
	res, err := actor.Flush(context.Background(), pendingOf(f, 9, a, b))
	if err != nil {
		t.Fatal(err)
	}
	rec := WaveRecord{Pool: 9, Batches: []IntegrationBatch{res.Batch}, Members: map[int]MemberState{
		6: {Status: MemberComposed, PatchIDs: patchIDsOf(a)},
		7: {Status: MemberComposed, PatchIDs: patchIDsOf(b)},
		8: {Status: MemberComposed, PatchIDs: []string{"unlisted"}},
	}}
	actor.Records = func() RecordSet {
		return RecordSet{Listed: []ListedRecord{{Pool: 9, Path: "root/integration-9/deliverable-6.json", Record: a}, {Pool: 9, Path: "root/integration-9/deliverable-7.json", Record: b}}}
	}
	f.g.Git("update-ref", PrivateMainRef, a.Commits[0].SHA)

	out, err := actor.WaveEnd(context.Background(), rec)

	if err != nil {
		t.Fatal(err)
	}
	if !out.Members[6].Landed || out.Members[7].Landed || out.Members[8].Landed {
		t.Fatalf("members = %+v, want only cycle 6 landed", out.Members)
	}
	if rec.Members[6].Landed {
		t.Error("WaveEnd changed the record it was given")
	}
	if left := f.g.Git("for-each-ref", "refs/heads/integration"); left != "" {
		t.Errorf("batch refs left:\n%s", left)
	}
}

func TestShadowActor_WaveEndRecordsARefreshFailureAndAFailedDelete(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	actor := shadowActorFor(f, nil)
	actor.Refresh = func(context.Context) error { return errors.New("offline") }
	rec := WaveRecord{Pool: 9, Batches: []IntegrationBatch{{Index: 1, Branch: "integration/p9-b1"}}}
	actor.Composer.Repo.Git = func(ctx context.Context, dir, stdin string, args ...string) (string, error) {
		if args[0] == "update-ref" {
			return "", errors.New("locked")
		}
		return f.repo.Git(ctx, dir, stdin, args...)
	}

	out, err := actor.WaveEnd(context.Background(), rec)

	if err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("err = %v, want the failed delete", err)
	}
	if len(out.Failures) != 1 || out.Failures[0].Step != "refresh" {
		t.Fatalf("failures = %+v", out.Failures)
	}
}

func TestShadowActor_TheClockAndPublisherDefaultSafely(t *testing.T) {
	t.Parallel()
	var a ShadowActor
	if a.now().IsZero() {
		t.Error("no clock gave the zero time")
	}
	a.emit(signalcenter.Event{Cycle: 1, Kind: signalcenter.KindShipDelivered, Code: CodeComposed, Severity: signalcenter.SeverityInfo, Reason: "r"})
	if got := a.listed(); len(got.Listed) != 0 {
		t.Errorf("listed = %+v", got)
	}
}

func TestShadowActor_AMemberWithNoCommitsIsNeverLanded(t *testing.T) {
	t.Parallel()
	landed, err := ShadowActor{}.landed(context.Background(), DeliverableRecord{})
	if landed || err != nil {
		t.Fatalf("landed = %v, %v, want false and no error", landed, err)
	}
}

//go:build acs

package cycle2414

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	baseSHA   = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	tipSHA    = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	briefFile = "/briefs/brief-"
	shipWave  = "wave-2414"
)

type stubActor struct{ integration.Actor }

type scriptedCI struct {
	verdict integration.Verdict
	opened  int
}

func (c *scriptedCI) Open(_ context.Context, b integration.Batch) (integration.PR, error) {
	c.opened++
	return integration.PR{Number: 900 + c.opened, Tip: b.Tip}, nil
}

func (c *scriptedCI) Watch(context.Context, integration.PR) (integration.Verdict, error) {
	return c.verdict, nil
}

func (c *scriptedCI) Rerun(context.Context, integration.PR) error { return nil }

type scriptedMain struct {
	head      string
	forwarded []string
}

func (m *scriptedMain) Head(context.Context) (string, error) { return m.head, nil }

func (m *scriptedMain) FastForward(_ context.Context, tip string) error {
	m.forwarded = append(m.forwarded, tip)
	m.head = tip
	return nil
}

type ledger struct {
	promoted  []int
	requeued  map[int]string
	briefed   []int
	synced    int
	syncErr   error
	requeueEr error
}

func (l *ledger) Promote(_ context.Context, rec integration.DeliverableRecord) error {
	l.promoted = append(l.promoted, rec.Cycle)
	return nil
}

func (l *ledger) Write(_ context.Context, ej *integration.Ejection) (string, error) {
	l.briefed = append(l.briefed, ej.Cycle)
	return briefFile + string(rune('0'+ej.Cycle)) + ".txt", nil
}

func (l *ledger) Requeue(_ context.Context, rec integration.DeliverableRecord, briefPath string) error {
	if l.requeued == nil {
		l.requeued = map[int]string{}
	}
	l.requeued[rec.Cycle] = briefPath
	return l.requeueEr
}

func (l *ledger) Sync(context.Context) (integration.SyncOutcome, error) {
	l.synced++
	return integration.SyncMoved, l.syncErr
}

func members(cycles ...int) []integration.DeliverableRecord {
	var recs []integration.DeliverableRecord
	for _, c := range cycles {
		recs = append(recs, integration.DeliverableRecord{Cycle: c})
	}
	return recs
}

func enforceActor(l *ledger, ci *scriptedCI, main *scriptedMain, comp integration.Composition) integration.EnforceActor {
	return integration.EnforceActor{
		Merger: integration.Merger{
			CI:   ci,
			Main: main,
			Compose: func(context.Context, []integration.DeliverableRecord) (integration.Composition, error) {
				return comp, nil
			},
		},
		Plane:   l,
		Requeue: l,
		Briefs:  l,
		Promote: l,
	}
}

func pending(recs []integration.DeliverableRecord) integration.PendingBatch {
	return integration.PendingBatch{Pool: 3, Index: 1, Base: baseSHA, Trigger: integration.FlushTrigger("wave_end"), Members: recs}
}

func TestC2414_001_StageActorsRegistersTheEnforceActorOnlyWhenOneIsGiven(t *testing.T) {
	shadow, enforce := &stubActor{}, &stubActor{}
	both := integration.StageActors(shadow, enforce)
	if both[policy.FleetIntegrationShadow] != integration.Actor(shadow) || both[policy.FleetIntegrationEnforce] != integration.Actor(enforce) {
		t.Errorf("StageActors(shadow, enforce) = %v, want shadow and enforce each under its stage", both)
	}
	shadowOnly := integration.StageActors(shadow, nil)
	if _, ok := shadowOnly[policy.FleetIntegrationEnforce]; ok {
		t.Errorf("a nil enforce actor was registered at the enforce stage: %v", shadowOnly)
	}
	if len(shadowOnly) != 1 || shadowOnly[policy.FleetIntegrationShadow] != integration.Actor(shadow) {
		t.Errorf("StageActors(shadow, nil) = %v, want only the shadow stage", shadowOnly)
	}
}

func TestC2414_002_FlushLandsTheBatchByFastForwardAndPromotesEachMember(t *testing.T) {
	l, ci, main := &ledger{}, &scriptedCI{verdict: integration.Green}, &scriptedMain{head: baseSHA}
	recs := members(1, 2)
	a := enforceActor(l, ci, main, integration.Composition{Batch: integration.Batch{Base: baseSHA, Tip: tipSHA}, Kept: recs})
	res, err := a.Flush(context.Background(), pending(recs))
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if !reflect.DeepEqual(main.forwarded, []string{tipSHA}) {
		t.Errorf("main fast-forwards = %v, want one to %s", main.forwarded, tipSHA)
	}
	for _, c := range []int{1, 2} {
		if got := res.Members[c]; got.Status != integration.MemberMerged || !got.Landed {
			t.Errorf("member %d = %+v, want merged and landed", c, got)
		}
	}
	slices.Sort(l.promoted)
	if !reflect.DeepEqual(l.promoted, []int{1, 2}) {
		t.Errorf("promoted = %v, want [1 2]", l.promoted)
	}
	if len(l.requeued) != 0 {
		t.Errorf("a clean landing requeued %v", l.requeued)
	}
	if res.Batch.Tip != tipSHA {
		t.Errorf("batch tip = %q, want %q", res.Batch.Tip, tipSHA)
	}
}

func TestC2414_003_FlushEjectsAConflictWithItsBriefAndLandsTheRest(t *testing.T) {
	l, ci, main := &ledger{}, &scriptedCI{verdict: integration.Green}, &scriptedMain{head: baseSHA}
	recs := members(1, 2)
	ej := &integration.Ejection{Cycle: 2, Cause: integration.CauseConflict, Paths: []string{"b.txt"}}
	a := enforceActor(l, ci, main, integration.Composition{
		Batch: integration.Batch{Base: baseSHA, Tip: tipSHA}, Kept: recs[:1], Ejected: []*integration.Ejection{ej},
	})
	res, err := a.Flush(context.Background(), pending(recs))
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got := res.Members[2]; got.Status != integration.MemberEjected || got.Landed {
		t.Errorf("ejected member = %+v, want ejected and not landed", got)
	}
	if got := res.Members[1]; got.Status != integration.MemberMerged || !got.Landed {
		t.Errorf("kept member = %+v, want merged and landed", got)
	}
	if !reflect.DeepEqual(l.briefed, []int{2}) {
		t.Errorf("briefs written for %v, want [2]", l.briefed)
	}
	if want := briefFile + "2.txt"; l.requeued[2] != want {
		t.Errorf("requeue of cycle 2 carries brief %q, want %q", l.requeued[2], want)
	}
	if !reflect.DeepEqual(l.promoted, []int{1}) {
		t.Errorf("promoted = %v, want only the landed member [1]", l.promoted)
	}
	if !reflect.DeepEqual(main.forwarded, []string{tipSHA}) {
		t.Errorf("main fast-forwards = %v, want one to %s", main.forwarded, tipSHA)
	}
}

func TestC2414_004_ARedCIEjectsTheMemberWithABriefAndNeverMovesMain(t *testing.T) {
	l, ci, main := &ledger{}, &scriptedCI{verdict: integration.Red}, &scriptedMain{head: baseSHA}
	recs := members(1)
	a := enforceActor(l, ci, main, integration.Composition{Batch: integration.Batch{Base: baseSHA, Tip: tipSHA}, Kept: recs})
	res, err := a.Flush(context.Background(), pending(recs))
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if len(main.forwarded) != 0 {
		t.Errorf("main moved to %v on a red CI", main.forwarded)
	}
	if got := res.Members[1]; got.Status != integration.MemberEjected || got.Landed {
		t.Errorf("member = %+v, want ejected and not landed", got)
	}
	if !reflect.DeepEqual(l.briefed, []int{1}) || l.requeued[1] == "" {
		t.Errorf("briefed=%v requeued=%v, want a brief and a requeue for cycle 1", l.briefed, l.requeued)
	}
	if len(l.promoted) != 0 {
		t.Errorf("promoted %v on a red CI", l.promoted)
	}
}

func TestC2414_005_AFailedRequeueIsReportedAndTheMemberIsNeverMarkedMerged(t *testing.T) {
	l, ci, main := &ledger{requeueEr: errors.New("inbox is unwritable")}, &scriptedCI{verdict: integration.Red}, &scriptedMain{head: baseSHA}
	recs := members(1)
	a := enforceActor(l, ci, main, integration.Composition{Batch: integration.Batch{Base: baseSHA, Tip: tipSHA}, Kept: recs})
	res, err := a.Flush(context.Background(), pending(recs))
	if err == nil && len(res.Failures) == 0 {
		t.Errorf("a failed requeue was swallowed: err=nil failures=none")
	}
	if got := res.Members[1]; got.Status == integration.MemberMerged || got.Landed {
		t.Errorf("member = %+v after a failed requeue, want neither merged nor landed", got)
	}
}

func TestC2414_006_ResumeDeletesOnlyTheLeakedRefsOfItsPoolAndNeverMovesMain(t *testing.T) {
	fx := gittest.Fixture(t)
	fx.Write("a.txt", "one\n")
	fx.Git("add", "-A")
	fx.Git("commit", "-q", "-m", "seed")
	head := fx.Git("rev-parse", "HEAD")
	for _, b := range []string{"integration/p3-b1", "integration/p3-b2", "integration/p4-b1"} {
		fx.Git("branch", b)
	}
	a := integration.EnforceActor{Repo: integration.Repo{Dir: fx.Dir, Git: integration.SysGit()}}
	rec := integration.WaveRecord{Pool: 3, Epoch: 1, ShadowBase: head}
	out, err := a.Resume(context.Background(), rec)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if out.Pool != 3 {
		t.Errorf("Resume returned pool %d, want 3", out.Pool)
	}
	left := strings.Fields(fx.Git("for-each-ref", "--format=%(refname:short)", "refs/heads/integration/"))
	if !reflect.DeepEqual(left, []string{"integration/p4-b1"}) {
		t.Errorf("integration refs left = %v, want only the other pool's integration/p4-b1", left)
	}
	if got := fx.Git("rev-parse", "main"); got != head {
		t.Errorf("main moved to %s on resume, want %s", got, head)
	}
}

func TestC2414_007_WaveEndSyncsThePlaneOnceAfterTheLastBatch(t *testing.T) {
	l := &ledger{}
	a := integration.EnforceActor{Plane: l, Promote: l, Requeue: l, Briefs: l}
	rec := integration.WaveRecord{Pool: 3, Epoch: 1, ShadowBase: baseSHA, Members: map[int]integration.MemberState{5: {Status: integration.MemberMerged, Landed: true}}}
	out, err := a.WaveEnd(context.Background(), rec)
	if err != nil {
		t.Fatalf("WaveEnd: %v", err)
	}
	if l.synced != 1 {
		t.Errorf("plane synced %d times, want exactly 1", l.synced)
	}
	if got := out.Members[5]; got.Status != integration.MemberMerged || !got.Landed {
		t.Errorf("a merged member changed to %+v at wave end", got)
	}
}

func TestC2414_008_WaveEndSurfacesAPlaneSyncFailure(t *testing.T) {
	sentinel := errors.New("plane sync refused")
	l := &ledger{syncErr: sentinel}
	a := integration.EnforceActor{Plane: l, Promote: l, Requeue: l, Briefs: l}
	rec := integration.WaveRecord{Pool: 3, Epoch: 1, ShadowBase: baseSHA}
	out, err := a.WaveEnd(context.Background(), rec)
	if !errors.Is(err, sentinel) && !hasFailure(out, sentinel.Error()) {
		t.Errorf("a failed plane sync was swallowed: err=%v failures=%v", err, out.Failures)
	}
}

func hasFailure(rec integration.WaveRecord, text string) bool {
	return slices.ContainsFunc(rec.Failures, func(f integration.DriverFailure) bool { return strings.Contains(f.Error, text) })
}

func deliverable(cycle int, scope string) integration.DeliverableRecord {
	return integration.DeliverableRecord{
		Wave:    shipWave,
		Cycle:   cycle,
		Trailer: integration.CycleTrailer(cycle),
		Commits: []integration.CommitIdentity{{SHA: "a1", BaseSHA: "b0", PatchID: "p1"}},
		Audit:   integration.AuditBinding{TreeSHA: "tree1", ArtifactSHA: "art1"},
		Scope:   []string{scope},
		Risk:    "low",

		DeliveredAt: time.Date(2026, 11, 14, 7, 0, 0, 0, time.UTC),
	}
}

func TestC2414_009_WriteOnceRefusesAnExistingRecordAndLeavesItsBytes(t *testing.T) {
	dir := t.TempDir()
	first := deliverable(42, "go/internal/x")
	path, err := integration.WriteRecordWith(dir, first, integration.WriteOnce)
	if err != nil {
		t.Fatalf("first write-once: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = integration.WriteRecordWith(dir, deliverable(42, "go/internal/other"), integration.WriteOnce)
	if !errors.Is(err, integration.ErrRecordExists) {
		t.Fatalf("second write-once error = %v, want ErrRecordExists", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Errorf("a refused write-once changed the record:\n%s\n--\n%s", before, after)
	}
	if _, err := integration.WriteRecordWith(dir, deliverable(43, "go/internal/y"), integration.WriteOnce); err != nil {
		t.Errorf("write-once of a different cycle: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Errorf("dir holds %d entries, want the 2 records and no temp file", len(entries))
	}
}

func TestC2414_010_ReplaceExistingKeepsTodaysOverwriteForShadowDelivery(t *testing.T) {
	dir := t.TempDir()
	if _, err := integration.WriteRecordWith(dir, deliverable(42, "go/internal/x"), integration.ReplaceExisting); err != nil {
		t.Fatal(err)
	}
	path, err := integration.WriteRecordWith(dir, deliverable(42, "go/internal/other"), integration.ReplaceExisting)
	if err != nil {
		t.Fatalf("replace-existing second write: %v", err)
	}
	got, err := integration.ReadRecord(path)
	if err != nil || !reflect.DeepEqual(got.Scope, []string{"go/internal/other"}) {
		t.Errorf("replaced record scope = %v err=%v, want the second write", got.Scope, err)
	}
	if _, err := integration.WriteRecord(dir, deliverable(42, "go/internal/third")); err != nil {
		t.Errorf("WriteRecord must keep overwriting: %v", err)
	}
}

func shipOptions(l shiptest.Lane, stage string) ship.Options {
	var stdin bytes.Reader
	return ship.Options{
		Class: ship.ClassCycle, CommitMessage: "evolve-cycle 1830", CycleID: shiptest.LandingCycle, RunID: shiptest.LandingRunID, AuditRound: 1,
		ActiveWorktree: l.WT, WorktreeBaseSHA: l.Base, WorkspacePath: l.WS, ProjectRoot: l.Repo, PluginRoot: l.Repo,
		ShipBinaryPath:                  filepath.Join(l.Repo, "ship-binary-fixture"),
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion, BuildExplanation: l.View,
		RequireBuildExplanationHandoff: true, Sleep: func(time.Duration) {},
		Stdin: &stdin, Stdout: io.Discard, Stderr: io.Discard,
		Integration: &ship.Integration{Stage: stage, Wave: shipWave, Risk: "low", InboxRank: 7, Scope: []string{"fixture.txt"}},
		NowFn:       func() ship.Now { return ship.Now{Unix: 1793000000, RFC3339: "2026-11-03T07:33:20Z"} },
	}
}

func seedDelivered(t *testing.T, l shiptest.Lane) (string, []byte) {
	t.Helper()
	path, err := integration.WriteRecord(integration.WaveDir(l.Repo, shipWave), deliverable(shiptest.LandingCycle, "previous-delivery"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, raw
}

func TestC2414_011_AnEnforceShipRefusesToOverwriteADeliveredRecord(t *testing.T) {
	l := shiptest.LaneReadyToLand(t)
	path, seeded := seedDelivered(t, l)
	res, err := ship.Run(context.Background(), shipOptions(l, policy.FleetIntegrationEnforce))
	if err == nil && res.ExitCode == ship.ExitOK {
		t.Errorf("an enforce ship over an existing record succeeded: logs=%v", res.Logs)
	}
	now, _ := os.ReadFile(path)
	if !bytes.Equal(seeded, now) {
		t.Errorf("the delivered record was overwritten, %d bytes became %d", len(seeded), len(now))
	}
}

func TestC2414_012_AShadowShipStillReplacesAnExistingRecord(t *testing.T) {
	l := shiptest.LaneReadyToLand(t)
	path, seeded := seedDelivered(t, l)
	res, err := ship.Run(context.Background(), shipOptions(l, policy.FleetIntegrationShadow))
	if err != nil || res.ExitCode != ship.ExitOK {
		t.Fatalf("shadow ship: exit=%v err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	now, _ := os.ReadFile(path)
	if bytes.Equal(seeded, now) {
		t.Errorf("a shadow ship left the old record in place")
	}
}

func TestC2414_013_PrefixCheckIsGoneFromTheComposer(t *testing.T) {
	if _, ok := reflect.TypeOf(integration.Composer{}).MethodByName("PrefixCheck"); ok {
		t.Errorf("Composer still has PrefixCheck; the enforce path composes through the lane batcher")
	}
}

type originPair struct {
	origin *gittest.Repo
	local  *gittest.Repo
	other  *gittest.Repo
}

func newOriginPair(t *testing.T) originPair {
	t.Helper()
	seed := gittest.Fixture(t)
	seed.Write("a.txt", "one\n")
	seed.Write("b.txt", "one\n")
	seed.Git("add", "-A")
	seed.Git("commit", "-q", "-m", "seed")
	origin := gittest.Bare(t)
	seed.Git("push", "-q", origin.Dir, "main")
	return originPair{origin: origin, local: gittest.Clone(t, origin.Dir), other: gittest.Clone(t, origin.Dir)}
}

func (p originPair) advanceOrigin(t *testing.T, rel, body string) {
	t.Helper()
	p.other.Write(rel, body)
	p.other.Git("add", "-A")
	p.other.Git("commit", "-q", "-m", "console merge "+rel)
	p.other.Git("push", "-q", "origin", "main")
}

func (p originPair) commitLocal(rel, body string) {
	p.local.Write(rel, body)
	p.local.Git("add", "-A")
	p.local.Git("commit", "-q", "-m", "local bookkeeping "+rel)
}

func (p originPair) sync(t *testing.T) integration.SyncOutcome {
	t.Helper()
	out, err := integration.SyncToOrigin(context.Background(), integration.Repo{Dir: p.local.Dir, Git: integration.SysGit()})
	if err != nil {
		t.Fatalf("SyncToOrigin: %v", err)
	}
	return out
}

func TestC2414_014_SyncToOriginReportsCurrentThenMovedAndFastForwardsThePlane(t *testing.T) {
	p := newOriginPair(t)
	if got := p.sync(t); got != integration.SyncCurrent {
		t.Errorf("an up-to-date plane = %q, want %q", got, integration.SyncCurrent)
	}
	p.advanceOrigin(t, "b.txt", "two\n")
	if got := p.sync(t); got != integration.SyncMoved {
		t.Errorf("a plane behind origin = %q, want %q", got, integration.SyncMoved)
	}
	if local, origin := p.local.Git("rev-parse", "main"), p.origin.Git("rev-parse", "main"); local != origin {
		t.Errorf("plane main %s != origin main %s after Moved", local, origin)
	}
}

func TestC2414_015_SyncToOriginTellsAheadFromDivergedAndMovesNeither(t *testing.T) {
	p := newOriginPair(t)
	p.commitLocal("a.txt", "local\n")
	ahead := p.local.Git("rev-parse", "main")
	if got := p.sync(t); got != integration.SyncAhead {
		t.Errorf("a plane with a local-only commit = %q, want %q", got, integration.SyncAhead)
	}
	p.advanceOrigin(t, "b.txt", "two\n")
	if got := p.sync(t); got != integration.SyncDiverged {
		t.Errorf("a plane with a local commit and a moved origin = %q, want %q", got, integration.SyncDiverged)
	}
	if got := p.local.Git("rev-parse", "main"); got != ahead {
		t.Errorf("the plane moved from %s to %s on a diverged sync", ahead, got)
	}
}

func TestC2414_016_SyncToOriginBlockedByDirtyWorkKeepsTheOperatorEditsAndNeverStashes(t *testing.T) {
	p := newOriginPair(t)
	p.local.Write("a.txt", "operator edit\n")
	head := p.local.Git("rev-parse", "main")
	p.advanceOrigin(t, "a.txt", "console change\n")
	if got := p.sync(t); got != integration.SyncBlocked {
		t.Errorf("a dirty plane under an incoming change = %q, want %q", got, integration.SyncBlocked)
	}
	raw, err := os.ReadFile(filepath.Join(p.local.Dir, "a.txt"))
	if err != nil || string(raw) != "operator edit\n" {
		t.Errorf("a.txt = %q err=%v, want the operator edit kept", raw, err)
	}
	if got := p.local.Git("rev-parse", "main"); got != head {
		t.Errorf("a blocked sync moved the plane from %s to %s", head, got)
	}
	if stash := p.local.Git("stash", "list"); stash != "" {
		t.Errorf("a blocked sync stashed the operator work: %q", stash)
	}
}

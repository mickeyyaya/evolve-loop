package core

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

type stageRecord struct {
	key    string
	fields map[string]string
}

type stageRecorderFake struct {
	mu      sync.Mutex
	records []stageRecord
	closed  int
}

func (r *stageRecorderFake) Record(key string, fields map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, stageRecord{key: key, fields: fields})
}

func (r *stageRecorderFake) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed++
}

func (r *stageRecorderFake) closedCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

type stageSealRequest struct {
	scope              stageevidence.ScopeKey
	in                 stageevidence.SealInput
	recorderClosedOnce bool
	ctxErr             error
	hasDeadline        bool
}

type stageSealerFake struct {
	mu       sync.Mutex
	rec      *stageRecorderFake
	requests []stageSealRequest
	err      error
}

func (s *stageSealerFake) SealStageEvidence(ctx context.Context, scope stageevidence.ScopeKey, in stageevidence.SealInput) (stageevidence.Seal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, hasDeadline := ctx.Deadline()
	s.requests = append(s.requests, stageSealRequest{scope: scope, in: in, recorderClosedOnce: s.rec.closedCount() == 1, ctxErr: ctx.Err(), hasDeadline: hasDeadline})
	return stageevidence.Seal{}, s.err
}

func stageRig() (*stageRecorderFake, *stageSealerFake, Option) {
	rec := &stageRecorderFake{}
	sealer := &stageSealerFake{rec: rec}
	var port StageSealer = sealer
	return rec, sealer, WithStageEvidence(StageEvidence{Open: func(int) StageRecorder { return rec }, Sealer: port})
}

type cancelingRunner struct {
	cancel context.CancelFunc
}

func (r cancelingRunner) Name() string { return "scout" }

func (r cancelingRunner) Run(ctx context.Context, _ PhaseRequest) (PhaseResponse, error) {
	r.cancel()
	return PhaseResponse{}, ctx.Err()
}

type stageExitCase struct {
	name      string
	exit      string
	shipped   bool
	runners   func(cancel context.CancelFunc) map[Phase]PhaseRunner
	storage   func() *fakeStorage
	wantError bool
}

func stageExitCases() []stageExitCase {
	plain := func(context.CancelFunc) map[Phase]PhaseRunner { return buildRunners(nil) }
	return []stageExitCase{
		{name: "normal", exit: stageExitCompleted, shipped: true, runners: plain},
		{name: "abort", exit: stageExitFailed, wantError: true, runners: func(context.CancelFunc) map[Phase]PhaseRunner {
			r := buildRunners(nil)
			r[PhaseScout] = &fakeRunner{name: "scout", failErr: errors.New("scout crashed"), failUntil: 99}
			r[PhaseAudit] = &fakeRunner{name: "audit", failErr: errors.New("audit crashed"), failUntil: 99}
			return r
		}},
		{name: "deadline", exit: stageExitDeadline, wantError: true, runners: func(cancel context.CancelFunc) map[Phase]PhaseRunner {
			r := buildRunners(nil)
			r[PhaseScout] = cancelingRunner{cancel: cancel}
			r[PhaseAudit] = cancelingRunner{cancel: cancel}
			return r
		}},
		{name: "parked", exit: stageExitParked, wantError: true, runners: func(context.CancelFunc) map[Phase]PhaseRunner {
			r := buildRunners(nil)
			r[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTransient(85), failUntil: 99}
			r[PhaseAudit] = &fakeRunner{name: "audit", failErr: wrapTransient(85), failUntil: 99}
			return r
		}},
		{name: "failed-finalize", exit: stageExitFailed, wantError: true, runners: plain, storage: func() *fakeStorage {
			return &fakeStorage{failOnWriteState: true}
		}},
	}
}

func TestFailedCycleEvidenceIsSealedAndScored(t *testing.T) {
	prevHook := QuotaBoundaryCheckpointer
	t.Cleanup(func() { QuotaBoundaryCheckpointer = prevHook })
	QuotaBoundaryCheckpointer = func(CycleState, string, time.Time) error { return nil }
	for _, mode := range []string{"fresh", "resume"} {
		for _, tc := range stageExitCases() {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				rec, sealer, opt := stageRig()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				st := &fakeStorage{state: State{LastCycleNumber: 40}, cycleState: CycleState{CycleID: 41, WorkspacePath: t.TempDir()}}
				if mode == "resume" {
					st.state = State{}
				}
				if tc.storage != nil {
					st.failOnWriteState = true
				}
				o := NewOrchestrator(st, &fakeLedger{}, tc.runners(cancel), opt)
				var res CycleResult
				var err error
				if mode == "fresh" {
					res, err = o.RunCycle(ctx, CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
				} else {
					res, err = o.RunCycleFromPhase(ctx, CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"}, &ResumePoint{Phase: string(PhaseAudit), CycleID: 41})
				}
				if (err != nil) != tc.wantError {
					t.Fatalf("cycle error = %v, want error %v", err, tc.wantError)
				}
				if tc.shipped && !IsShippingVerdict(res.FinalVerdict) {
					t.Fatalf("the normal case ended %q, so it cannot prove a shipped seal", res.FinalVerdict)
				}
				if len(sealer.requests) != 1 {
					t.Fatalf("seals = %d, want exactly one on this exit path", len(sealer.requests))
				}
				got := sealer.requests[0]
				wantScope := stageevidence.CycleScope(res.Cycle)
				if mode == "resume" {
					wantScope = stageevidence.CycleScope(41)
				}
				if got.scope != wantScope || got.in.Exit != tc.exit || got.in.Shipped != tc.shipped {
					t.Errorf("seal = scope %v exit %q shipped %v, want %v %q %v", got.scope, got.in.Exit, got.in.Shipped, wantScope, tc.exit, tc.shipped)
				}
				if !got.recorderClosedOnce {
					t.Error("the seal ran before the session recorder was closed once")
				}
				if got.ctxErr != nil || !got.hasDeadline {
					t.Errorf("seal ctx err %v deadline %v, want a live bounded context even after the cycle context ended", got.ctxErr, got.hasDeadline)
				}
				if rec.closedCount() != 1 {
					t.Errorf("recorder closed %d times, want 1", rec.closedCount())
				}
			})
		}
	}
}

func TestStageSealFaultIsLoudAndNeverFailsTheCycle(t *testing.T) {
	t.Parallel()
	center, events := captureSignals()
	rec, sealer, opt := stageRig()
	sealer.err = errors.New("ledger is full")
	o := NewOrchestrator(&fakeStorage{state: State{}}, &fakeLedger{}, buildRunners(nil), opt, WithSignalCenter(center))
	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	if err != nil || !IsShippingVerdict(res.FinalVerdict) {
		t.Fatalf("a seal that failed changed the cycle: verdict %q err %v", res.FinalVerdict, err)
	}
	faults := eventsWithCode(*events, CodeStageEvidenceFault)
	if len(faults) != 1 || faults[0].Fields["cycle"] == "" || faults[0].Fields["error"] != "ledger is full" || rec.closedCount() != 1 {
		t.Errorf("fault events %+v closed %d, want one loud fault naming the cycle and the error", faults, rec.closedCount())
	}
}

func TestStageSealOfAnEmptyScopeIsQuiet(t *testing.T) {
	t.Parallel()
	center, events := captureSignals()
	_, sealer, opt := stageRig()
	sealer.err = stageevidence.ErrEmptyChain
	o := NewOrchestrator(&fakeStorage{state: State{}}, &fakeLedger{}, buildRunners(nil), opt, WithSignalCenter(center))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"}); err != nil {
		t.Fatal(err)
	}
	if faults := eventsWithCode(*events, CodeStageEvidenceFault); len(faults) != 0 || len(sealer.requests) != 1 {
		t.Errorf("faults %+v seals %d, want a cycle with no decisions sealed quietly once", faults, len(sealer.requests))
	}
}

func TestOrchestratorWithoutStageEvidenceRunsAsBefore(t *testing.T) {
	t.Parallel()
	o := NewOrchestrator(&fakeStorage{state: State{}}, &fakeLedger{}, buildRunners(nil))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"}); err != nil {
		t.Fatal(err)
	}
	o.recordStage(1, "k", nil)
	if o.StageEvidenceWired() {
		t.Error("an orchestrator built without stage evidence reports it wired")
	}
}

func TestStageEvidenceOptionIgnoresAHalfWiredPort(t *testing.T) {
	t.Parallel()
	rec, _, _ := stageRig()
	o := NewOrchestrator(&fakeStorage{state: State{}}, &fakeLedger{}, buildRunners(nil), WithStageEvidence(StageEvidence{Open: func(int) StageRecorder { return rec }}))
	if o.StageEvidenceWired() {
		t.Error("a port with no sealer was wired: the cycle would record and never seal")
	}
}

func TestLadderDecisionWritesEvidenceRecord(t *testing.T) {
	t.Parallel()
	rec, _, opt := stageRig()
	probe := newLadderProbe(0)
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", nil)
	runners := buildRunners(nil)
	runners[PhaseBuild] = probe
	o := NewOrchestrator(&fakeStorage{state: State{}}, &fakeLedger{}, runners, WithReviewer(probe), opt,
		WithRetryConfig(policy.RetryConfig{ContractCorrectionRetries: 1}), WithWorkflowConfig(ladderWorkflow(policy.RetryLadderShadow)))
	_, _ = o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true})
	if len(rec.records) != 1 || rec.records[0].key != StageKeyRetryLadder {
		t.Fatalf("records = %+v, want the one ladder decision of the shadow cycle", rec.records)
	}
	f := rec.records[0].fields
	if f["stage"] != policy.RetryLadderShadow || f["origin"] != originContractCorrection || f["rung"] != "R0" || f["next"] == "" || f["action"] != "raise" || f["would_stop"] != "false" {
		t.Errorf("fields = %v, want the fields of the signal and a would_stop of false for a raise", f)
	}
}

func TestStageExitOfNamesEachWayACycleEnds(t *testing.T) {
	t.Parallel()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []struct {
		name      string
		ctx       context.Context
		err       error
		completed bool
		want      string
	}{
		{"completed", context.Background(), nil, true, stageExitCompleted},
		{"failed", context.Background(), errors.New("boom"), false, stageExitFailed},
		{"completed then canceled in cleanup", canceled, nil, true, stageExitCompleted},
		{"canceled and failed", canceled, errors.New("boom"), true, stageExitDeadline},
		{"deadline beats failure", canceled, errors.New("boom"), false, stageExitDeadline},
		{"unended", context.Background(), nil, false, stageExitUnended},
	}
	for _, tc := range cases {
		if got := stageExitOf(tc.ctx, tc.err, tc.completed); got != tc.want {
			t.Errorf("%s: exit = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestStageRecordsGoOnlyIntoTheLiveSessionOfTheirCycle(t *testing.T) {
	t.Parallel()
	rec, _, opt := stageRig()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), opt)
	session := o.beginStageSession(7)

	o.recordStage(8, StageKeyRetryLadder, map[string]string{"x": "other cycle"})
	o.recordStage(7, StageKeyRetryLadder, map[string]string{"x": "own"})
	session.finish(context.Background(), nil, true, VerdictPASS)
	o.recordStage(7, StageKeyRetryLadder, map[string]string{"x": "after the seal"})

	if len(rec.records) != 1 || rec.records[0].fields["x"] != "own" {
		t.Fatalf("records = %+v, want only the own record of the live session", rec.records)
	}
}

func TestStageFaultEventNamesTheCycleTheActionAndTheCause(t *testing.T) {
	t.Parallel()
	ev := StageFaultEvent(9, "somewhere", "written", errors.New("disk full"))
	if ev.Code != CodeStageEvidenceFault || ev.Cycle != 9 || ev.Origin != "somewhere" || ev.Fields["cycle"] != "9" || ev.Fields["error"] != "disk full" || ev.Reason != "stage evidence of cycle 9 could not be written: disk full" {
		t.Errorf("event = %+v, want the stage fault code with cycle, origin, cause and the written reason", ev)
	}
}

func TestOverlappingCyclesOnOneOrchestratorKeepTheirOwnEvidence(t *testing.T) {
	t.Parallel()
	rec, _, opt := stageRig()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), opt)
	first := o.beginStageSession(7)
	second := o.beginStageSession(8)

	o.recordStage(7, StageKeyRetryLadder, map[string]string{"x": "first"})
	o.recordStage(8, StageKeyRetryLadder, map[string]string{"x": "second"})
	second.finish(context.Background(), nil, true, VerdictPASS)
	o.recordStage(7, StageKeyRetryLadder, map[string]string{"x": "first again"})
	first.finish(context.Background(), nil, true, VerdictPASS)

	if len(rec.records) != 3 || rec.records[2].fields["x"] != "first again" {
		t.Fatalf("records = %+v, want the three records of two overlapping live sessions, the first still live after the second sealed", rec.records)
	}
}

func TestACompletedCycleThatDidNotShipSealsUnshipped(t *testing.T) {
	t.Parallel()
	_, sealer, opt := stageRig()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), opt)

	o.beginStageSession(7).finish(context.Background(), nil, true, VerdictFAIL)

	if len(sealer.requests) != 1 || sealer.requests[0].in.Shipped || sealer.requests[0].in.Exit != stageExitCompleted {
		t.Fatalf("seal = %+v, want completed and not shipped", sealer.requests)
	}
}

type realSealerFake struct {
	working []byte
	seal    stageevidence.Seal
}

func (r *realSealerFake) SealStageEvidence(_ context.Context, scope stageevidence.ScopeKey, in stageevidence.SealInput) (stageevidence.Seal, error) {
	seal, _, err := stageevidence.BuildSeal(scope, r.working, in)
	r.seal = seal
	return seal, err
}

func TestAFreshSealDoesNotMeasureRoutesSoAHeldDecisionIsNeverScoredAsZeroConflicts(t *testing.T) {
	t.Parallel()
	const cycle = 7
	const version = "v1"
	held := stageevidence.Link("", stageevidence.Record{
		ID: cycle, Key: "held_proposals.stage", Version: version, Kind: stageevidence.KindDecision, Seq: 1,
		Fields: map[string]string{stageevidence.FieldWouldHold: "true", stageevidence.FieldItem: "item-a"},
	})
	closed := stageevidence.Link(held.Hash, stageevidence.Record{
		ID: cycle, Key: "held_proposals.stage", Version: version, Kind: stageevidence.KindClose, Seq: 2,
		Fields: map[string]string{"counter.held_proposals.stage": "1"},
	})
	var working []byte
	for _, r := range []stageevidence.Record{held, closed} {
		line, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		working = append(append(working, line...), '\n')
	}
	sealer := &realSealerFake{working: working}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithStageEvidence(StageEvidence{Open: func(int) StageRecorder { return &stageRecorderFake{} }, Sealer: sealer}))

	o.beginStageSession(cycle).finish(context.Background(), nil, true, VerdictFAIL)

	chain := stageevidence.Chain{Records: []stageevidence.Record{held, closed}, Outcomes: []stageevidence.Outcome{sealer.seal.Outcome(stageevidence.CycleScope(cycle))}}
	scored := stageevidence.Score(chain, version, gatecontract.Promotion{MinDecisions: 1}, stageevidence.DefaultScorer())
	if sealer.seal.RoutesMeasured || scored.Ready || scored.NotReady != stageevidence.ReasonConflictsUnmeasured {
		t.Errorf("routes measured = %t, ready = %t, not ready = %q; want unmeasured routes, not ready, %q", sealer.seal.RoutesMeasured, scored.Ready, scored.NotReady, stageevidence.ReasonConflictsUnmeasured)
	}
}

func TestReleaseRunSealsEvenWhenTheCleanupPanics(t *testing.T) {
	t.Parallel()
	_, sealer, opt := stageRig()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), opt)
	cr := &cycleRun{o: o, ctx: context.Background(), cycle: 9, stage: o.beginStageSession(9)}
	cr.cycleCompletedNormally = true
	cr.result.FinalVerdict = VerdictPASS
	func() {
		defer func() { _ = recover() }()
		cr.releaseRun(func(bool, bool) { panic("cleanup crashed") }, nil)
	}()
	if len(sealer.requests) != 1 || sealer.requests[0].in.Exit != stageExitCompleted || !sealer.requests[0].in.Shipped {
		t.Fatalf("seals = %+v, want one completed and shipped seal despite the panic", sealer.requests)
	}
}

func TestReleaseSealSealsEvenWhenTheShipWindowReleasePanics(t *testing.T) {
	t.Parallel()
	sealed := 0
	var o *Orchestrator
	func() {
		defer func() { _ = recover() }()
		o.releaseSeal(func() { sealed++ })
	}()
	if sealed != 1 {
		t.Fatalf("sealed %d times, want once although the ship-window release panicked", sealed)
	}
	live := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	var order []string
	live.releaseSeal(func() { order = append(order, "seal") })
	if len(order) != 1 {
		t.Fatalf("order = %v, want the seal to run once after a clean release", order)
	}
}

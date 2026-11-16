package core

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var composedRed = baseattr.Key{Package: "example.com/m/p", Test: "TestX"}

type composedBaseScript struct {
	red   []baseattr.Key
	err   error
	asked []baseattr.Reds
}

func (s *composedBaseScript) run(_ context.Context, _ string, r baseattr.Reds) ([]baseattr.Key, error) {
	s.asked = append(s.asked, r)
	return s.red, s.err
}

func composedRoute(t *testing.T, gates map[string]ciparity.GateOutcome, script *composedBaseScript) (Phase, bool, *composedPark, []signalcenter.Event, *carryHarness) {
	t.Helper()
	h := pendedIdenticalLane(t)
	mirrorMain(t, h.fx)
	h.gates = gates
	center, events := captureSignals()
	h.signals = center
	h.stderr = io.Discard
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{entries: []LedgerEntry{h.auditRow()}}, buildRunners(nil),
		WithSignalCenter(center),
		WithStderr(io.Discard),
		WithCompositionSnapshot(func(context.Context, string, string) (CompositionAuditSnapshot, error) {
			return CompositionAuditSnapshot{LaneAuditRef: "audit-ref"}, nil
		}),
		WithCompositionGateRunner(func(context.Context, string) map[string]ciparity.GateOutcome { return h.gates }),
		WithCompositionVerdictWriter(func(string, CompositionVerdictInput) error { return nil }),
	)
	o.composedBaseRun = script.run
	ctx, park := withComposedPark(context.Background())
	cs := h.fx.cycleState()
	next, recovering := o.routeRebasedExplanation(ctx, h.fx.root, unwindCycle, &cs)
	return next, recovering, park, *events, h
}

func mirrorMain(t *testing.T, fx *shippedLane) {
	t.Helper()
	gittest.At(t, fx.root).Git("fetch", "-q", "--update-head-ok", fx.worktree, "main:main")
}

func redTestGate() map[string]ciparity.GateOutcome {
	gates := allComposedGatesPass()
	gates["test"] = ciparity.GateOutcome{Status: "fail", Tail: "FAIL", Reds: baseattr.Reds{Keys: []baseattr.Key{composedRed}, Flags: []string{"-race"}}}
	return gates
}

func codesOf(events []signalcenter.Event) map[signalcenter.Code]int {
	out := map[signalcenter.Code]int{}
	for _, e := range events {
		out[e.Code]++
	}
	return out
}

func TestComposedGate_ARedThatMainAlsoFailsParksTheLane(t *testing.T) {
	t.Parallel()
	script := &composedBaseScript{red: []baseattr.Key{composedRed}}

	next, recovering, park, events, h := composedRoute(t, redTestGate(), script)

	if next != "" || recovering {
		t.Fatalf("route = (%q, %v), want no recovery: a red that main also has is not the lane's", next, recovering)
	}
	if !park.Parked() || len(park.Verdict.MainRed) != 1 || park.Verdict.MainRed[0] != composedRed {
		t.Errorf("park = %+v, want the verdict with the red on main", park)
	}
	if len(script.asked) != 1 || script.asked[0].Keys[0] != composedRed || len(script.asked[0].Flags) != 1 || script.asked[0].Flags[0] != "-race" {
		t.Errorf("base runs = %+v, want one run of the red test with the flags of the gate", script.asked)
	}
	codes := codesOf(events)
	if codes[CodeMainRedParked] != 1 || codes[CodeComposedGateDeclined] != 0 {
		t.Errorf("signals = %v, want the main-red park and no decline", codes)
	}
	stop, found, err := mainstop.Read(filepath.Join(h.fx.root, ".evolve"))
	if err != nil || !found || len(stop.Parked) != 1 {
		t.Errorf("stop = %+v, %v, %v, want one parked lane", stop, found, err)
	}
}

func TestComposedGate_ARedOnlyTheLaneHasGoesToReaudit(t *testing.T) {
	t.Parallel()
	script := &composedBaseScript{}

	next, recovering, park, events, h := composedRoute(t, redTestGate(), script)

	if !recovering || next != PhaseAudit || park.Parked() {
		t.Fatalf("route = (%q, %v), parked %v, want the re-audit of today", next, recovering, park.Parked())
	}
	codes := codesOf(events)
	if codes[CodeComposedGateDeclined] != 1 || codes[CodeMainRedParked] != 0 {
		t.Errorf("signals = %v, want the decline and no park", codes)
	}
	if _, found, _ := mainstop.Read(filepath.Join(h.fx.root, ".evolve")); found {
		t.Error("a lane-only red opened a main-red stop")
	}
}

func TestComposedGate_ABaseRunErrorKeepsTheRouteOfToday(t *testing.T) {
	t.Parallel()
	script := &composedBaseScript{err: errors.New("go test did not start")}

	next, recovering, park, events, _ := composedRoute(t, redTestGate(), script)

	if !recovering || next != PhaseAudit || park.Parked() {
		t.Fatalf("route = (%q, %v), parked %v, want the re-audit of today", next, recovering, park.Parked())
	}
	if codes := codesOf(events); codes[CodeComposedGateDeclined] != 1 || codes[CodeMainRedParked] != 0 {
		t.Errorf("signals = %v, want the decline and no park", codes)
	}
}

func TestComposedGate_ARedWithNoNamedTestIsNotRunOnMain(t *testing.T) {
	t.Parallel()
	script := &composedBaseScript{}
	gates := allComposedGatesPass()
	gates["compile"] = ciparity.GateOutcome{Status: "fail", Tail: "build failed"}

	next, recovering, park, _, _ := composedRoute(t, gates, script)

	if !recovering || next != PhaseAudit || park.Parked() || len(script.asked) != 0 {
		t.Fatalf("route = (%q, %v), parked %v, runs %d, want the re-audit and no base run", next, recovering, park.Parked(), len(script.asked))
	}
}

func TestComposedGate_OneRedOnEachSideParksOnlyWhenEveryGateIsRedOnMain(t *testing.T) {
	t.Parallel()
	script := &composedBaseScript{red: []baseattr.Key{composedRed}}
	gates := redTestGate()
	gates["acs"] = ciparity.GateOutcome{Status: "fail", Reds: baseattr.Reds{Keys: []baseattr.Key{{Package: "example.com/m/q", Test: "TestY"}}}}

	next, recovering, park, _, _ := composedRoute(t, gates, script)

	if !recovering || next != PhaseAudit || park.Parked() {
		t.Fatalf("route = (%q, %v), parked %v, want the re-audit: the acs red is not red on main", next, recovering, park.Parked())
	}
}

func TestComposedPark_TheCycleEndsWithTheMainRedParkError(t *testing.T) {
	t.Parallel()
	v := baseattr.Verdict{BaseRed: []baseattr.Key{composedRed}, MainRed: []baseattr.Key{composedRed}, Base: "b", Main: "m"}
	p := &composedPark{Verdict: v, parked: true}

	err := p.shipError()

	if !errors.Is(err, ErrMainRedParked) || err.Code != shiperr.CodeMainRedPark || baseattr.VerdictOfDebug(err.Debug).Main != "m" {
		t.Errorf("error = %+v, want the main-red park error with the verdict", err)
	}
}

func composedCycleRun(t *testing.T, fx *shippedLane, version int, script *composedBaseScript) *cycleRun {
	t.Helper()
	mirrorMain(t, fx)
	center, _ := captureSignals()
	h := &carryHarness{fx: fx, gates: redTestGate()}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{entries: []LedgerEntry{h.auditRow()}}, buildRunners(nil),
		WithSignalCenter(center),
		WithStderr(io.Discard),
		WithCompositionSnapshot(func(ctx context.Context, worktree, _ string) (CompositionAuditSnapshot, error) {
			diff, _, err := gitCapture(ctx, worktree, "diff", "main...HEAD")
			if err != nil {
				return CompositionAuditSnapshot{}, err
			}
			patchID, err := compositionPatchID([]byte(diff))
			return CompositionAuditSnapshot{LaneAuditRef: "audit-ref", Diff: []byte(diff), PatchID: patchID}, err
		}),
		WithScopedMergeReviewer(func([]MergeHunk, string, string) ScopedMergeReviewOutcome {
			return ScopedMergeReviewOutcome{Disposition: ScopedMergeCompatible}
		}),
		WithCompositionGateRunner(func(context.Context, string) map[string]ciparity.GateOutcome { return h.gates }),
		WithCompositionVerdictWriter(func(string, CompositionVerdictInput) error { return nil }),
	)
	o.composedBaseRun = script.run
	cs := fx.cycleState()
	cs.ExplanationDocumentationVersion = version
	return &cycleRun{o: o, ctx: context.Background(), cycle: unwindCycle, cs: cs, req: CycleRequest{ProjectRoot: fx.root}}
}

func TestComposedGate_ARedThatMainAlsoFailsEndsTheCycleAsParkedOnEachRebaseRoute(t *testing.T) {
	t.Parallel()
	for name, version := range map[string]int{"the identity carry": 1, "the scoped merge review": 0} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fx := newShippedLane(t, "lane.txt", addLaneFile, addPeerFile)
			script := &composedBaseScript{red: []baseattr.Key{composedRed}}
			cr := composedCycleRun(t, fx, version, script)
			raised := NewShipError(CodeGitFleetRebaseNeeded, ShipClassTransient, StageAtomicShip, "peer moved main")

			recovered := cr.recoverShipError(PhaseShip, raised, PhaseResponse{}, 1)
			err := cr.parkedOr(raised)

			if recovered || !errors.Is(err, ErrMainRedParked) || !isParked(err) {
				t.Fatalf("recovered %v, error %v, want a parked cycle and no re-audit", recovered, err)
			}
			if len(script.asked) != 1 {
				t.Errorf("base runs = %d, want 1", len(script.asked))
			}
		})
	}
}

func TestComposedPark_AnUnparkedCycleKeepsItsError(t *testing.T) {
	t.Parallel()
	raised := errors.New("ship failed")
	cr := &cycleRun{}

	if got := cr.parkedOr(raised); got != raised {
		t.Errorf("error = %v, want the raised error", got)
	}
}

func TestComposedPark_ADefaultBaseRunnerRunsGoTest(t *testing.T) {
	t.Parallel()
	if NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil)).composedBaseRunner() == nil {
		t.Error("the default base runner is nil")
	}
}

func TestComposedGate_AResumedShipThatParksReturnsTheParkError(t *testing.T) {
	t.Parallel()
	fx := newShippedLane(t, "lane.txt", addLaneFile, addPeerFile)
	script := &composedBaseScript{red: []baseattr.Key{composedRed}}
	cr := composedCycleRun(t, fx, 1, script)
	raised := NewShipError(CodeGitFleetRebaseNeeded, ShipClassTransient, StageAtomicShip, "peer moved main")

	next, recovering, err := cr.recoverResumedShip(PhaseShip, raised, PhaseResponse{}, 1)

	if next != "" || recovering || !errors.Is(err, ErrMainRedParked) {
		t.Errorf("recoverResumedShip = (%q, %v, %v), want the park error", next, recovering, err)
	}
}

func TestDispatch_AShipThatParksInComposedGatesReturnsTheMainRedParkError(t *testing.T) {
	t.Parallel()
	fx := newShippedLane(t, "lane.txt", addLaneFile, addPeerFile)
	script := &composedBaseScript{red: []baseattr.Key{composedRed}}
	cr := composedCycleRun(t, fx, 1, script)
	cr.o.runners[PhaseShip] = &fakeRunner{
		name:      "ship",
		failErr:   NewShipError(CodeGitFleetRebaseNeeded, ShipClassTransient, StageAtomicShip, "peer moved main"),
		failUntil: 99,
	}

	_, act, err := cr.dispatch(PhaseShip)

	if act != loopAbort || !errors.Is(err, ErrMainRedParked) || !isParked(err) {
		t.Fatalf("dispatch = (%v, %v), want loopAbort with ErrMainRedParked", act, err)
	}
}

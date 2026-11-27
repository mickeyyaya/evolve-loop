package core

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleidentity"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var (
	idStart = cycleidentity.Identity{Graph: "g1", Contract: "c1", Policy: "p1"}
)

type identityRig struct {
	o      *Orchestrator
	ledger *fakeLedger
	stderr *bytes.Buffer
	center *signalcenter.Center
	now    cycleidentity.Identity
}

func newIdentityRig(t *testing.T, stage config.Stage) *identityRig {
	t.Helper()
	rig := &identityRig{ledger: &fakeLedger{}, stderr: &bytes.Buffer{}, center: signalcenter.New(), now: idStart}
	rig.o = NewOrchestrator(&fakeStorage{}, rig.ledger, nil,
		WithStderr(rig.stderr), WithSignalCenter(rig.center),
		WithResumeIdentity(ResumeIdentityConfig{Stage: stage, Compute: func(string) (cycleidentity.Identity, error) { return rig.now, nil }}))
	return rig
}

func (r *identityRig) start(t *testing.T, cycle int) {
	t.Helper()
	r.o.recordCycleIdentity(context.Background(), "", cycle)
}

func (r *identityRig) shadowSignals() int {
	n := 0
	for _, e := range r.center.Recent() {
		if e.Code == CodeResumeIdentityShadow {
			n++
		}
	}
	return n
}

func TestRecordCycleIdentity_WritesTheIdentityAsTheLedgerRecord(t *testing.T) {
	t.Parallel()
	rig := newIdentityRig(t, config.StageShadow)
	rig.start(t, 7)
	if len(rig.ledger.entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(rig.ledger.entries))
	}
	e := rig.ledger.entries[0]
	got, err := cycleidentity.Decode(e.Message)
	if e.Kind != LedgerKindCycleIdentity || e.Cycle != 7 || err != nil || got != idStart {
		t.Fatalf("entry = %+v, identity %+v, err %v", e, got, err)
	}
}

func TestRecordCycleIdentity_SkipsWhenOffUnwiredOrFailing(t *testing.T) {
	t.Parallel()
	off := newIdentityRig(t, config.StageOff)
	off.start(t, 1)
	unwiredLedger := &fakeLedger{}
	unwired := NewOrchestrator(&fakeStorage{}, unwiredLedger, nil)
	unwired.recordCycleIdentity(context.Background(), "", 1)
	failing := newIdentityRig(t, config.StageShadow)
	failing.o.resumeIdentity.Compute = func(string) (cycleidentity.Identity, error) { return cycleidentity.Identity{}, errors.New("boom") }
	failing.start(t, 1)
	appendFail := newIdentityRig(t, config.StageShadow)
	appendFail.ledger.failOnAppend = true
	appendFail.start(t, 1)
	for name, n := range map[string]int{"off": len(off.ledger.entries), "unwired": len(unwiredLedger.entries), "compute error": len(failing.ledger.entries), "append error": len(appendFail.ledger.entries)} {
		if n != 0 {
			t.Errorf("%s: entries = %d, want 0", name, n)
		}
	}
	if !strings.Contains(failing.stderr.String(), "WARN cycle identity") || !strings.Contains(appendFail.stderr.String(), "ledger append") {
		t.Errorf("missing WARN: %q %q", failing.stderr.String(), appendFail.stderr.String())
	}
}

func TestVerifyResumeIdentity_ByStageAndChange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		stage      config.Stage
		change     cycleidentity.Identity
		accept     bool
		wantParts  []cycleidentity.Part
		wantRefuse bool
		wantSignal int
		wantRecord int
	}{
		{name: "same identity", stage: config.StageEnforce, change: idStart},
		{name: "graph at enforce refuses", stage: config.StageEnforce, change: cycleidentity.Identity{Graph: "g2", Contract: "c1", Policy: "p1"}, wantParts: []cycleidentity.Part{cycleidentity.PartGraph}, wantRefuse: true},
		{name: "graph at shadow signals and proceeds", stage: config.StageShadow, change: cycleidentity.Identity{Graph: "g2", Contract: "c1", Policy: "p1"}, wantSignal: 1},
		{name: "contract at enforce refuses", stage: config.StageEnforce, change: cycleidentity.Identity{Graph: "g1", Contract: "c2", Policy: "p1"}, wantParts: []cycleidentity.Part{cycleidentity.PartContract}, wantRefuse: true},
		{name: "policy at enforce refuses", stage: config.StageEnforce, change: cycleidentity.Identity{Graph: "g1", Contract: "c1", Policy: "p2"}, wantParts: []cycleidentity.Part{cycleidentity.PartPolicy}, wantRefuse: true},
		{name: "migration at enforce proceeds and records", stage: config.StageEnforce, change: cycleidentity.Identity{Graph: "g2", Contract: "c1", Policy: "p1"}, accept: true, wantRecord: 1},
		{name: "migration at shadow records and sends no signal", stage: config.StageShadow, change: cycleidentity.Identity{Graph: "g2", Contract: "c1", Policy: "p1"}, accept: true, wantRecord: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rig := newIdentityRig(t, tc.stage)
			rig.start(t, 3)
			rig.now = tc.change
			err := rig.o.verifyResumeIdentity(context.Background(), CycleRequest{AcceptIdentityChange: tc.accept}, 3)
			var mm *cycleidentity.MismatchError
			if tc.wantRefuse != (err != nil) || (tc.wantRefuse && (!errors.Is(err, cycleidentity.ErrMismatch) || !errors.As(err, &mm) || len(mm.Parts) != 1 || mm.Parts[0] != tc.wantParts[0])) {
				t.Fatalf("err = %v, want refuse=%v parts=%v", err, tc.wantRefuse, tc.wantParts)
			}
			if got := rig.shadowSignals(); got != tc.wantSignal {
				t.Errorf("shadow signals = %d, want %d", got, tc.wantSignal)
			}
			overrides := 0
			for _, e := range rig.ledger.entries {
				if e.Kind == LedgerKindCycleIdentityOverride && strings.Contains(e.Message, "changed: graph") {
					overrides++
				}
			}
			if overrides != tc.wantRecord {
				t.Errorf("override records = %d, want %d", overrides, tc.wantRecord)
			}
		})
	}
}

func TestVerifyResumeIdentity_SameIdentityEmitsNoSignal(t *testing.T) {
	t.Parallel()
	rig := newIdentityRig(t, config.StageShadow)
	rig.start(t, 3)
	if err := rig.o.verifyResumeIdentity(context.Background(), CycleRequest{}, 3); err != nil || rig.shadowSignals() != 0 {
		t.Fatalf("err %v signals %d", err, rig.shadowSignals())
	}
}

func TestVerifyResumeIdentity_NoRecordedIdentityWarnsOnceAtEveryStage(t *testing.T) {
	t.Parallel()
	for _, stage := range []config.Stage{config.StageShadow, config.StageEnforce} {
		rig := newIdentityRig(t, stage)
		if err := rig.o.verifyResumeIdentity(context.Background(), CycleRequest{}, 9); err != nil {
			t.Fatalf("%s: %v", stage, err)
		}
		if n := strings.Count(rig.stderr.String(), "WARN"); n != 1 {
			t.Errorf("%s: WARN lines = %d, want 1: %q", stage, n, rig.stderr.String())
		}
	}
}

func TestVerifyResumeIdentity_OffAndUnwiredAreSilent(t *testing.T) {
	t.Parallel()
	off := newIdentityRig(t, config.StageOff)
	unwired := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, nil)
	for _, o := range []*Orchestrator{off.o, unwired} {
		if err := o.verifyResumeIdentity(context.Background(), CycleRequest{}, 1); err != nil {
			t.Fatal(err)
		}
	}
}

type identityBrokenLedger struct {
	fakeLedger
	iterErr error
	nextErr error
}

func (b *identityBrokenLedger) Iter(context.Context) (LedgerIterator, error) {
	if b.iterErr != nil {
		return nil, b.iterErr
	}
	return errIterator{b.nextErr}, nil
}

type errIterator struct{ err error }

func (e errIterator) Next() (LedgerEntry, bool, error) { return LedgerEntry{}, false, e.err }
func (e errIterator) Close() error                     { return nil }

func TestVerifyResumeIdentity_UnverifiableIdentityRefusesOnlyAtEnforce(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	causes := map[string]func(*identityRig){
		"iter error": func(r *identityRig) { r.o.ledger = &identityBrokenLedger{iterErr: boom} },
		"next error": func(r *identityRig) { r.o.ledger = &identityBrokenLedger{nextErr: boom} },
		"decode error": func(r *identityRig) {
			r.ledger.entries = []LedgerEntry{{Kind: LedgerKindCycleIdentity, Cycle: 4, Message: "{"}}
		},
		"compute error": func(r *identityRig) {
			r.start(t, 4)
			r.o.resumeIdentity.Compute = func(string) (cycleidentity.Identity, error) { return cycleidentity.Identity{}, boom }
		},
	}
	for name, arrange := range causes {
		for _, stage := range []config.Stage{config.StageShadow, config.StageEnforce} {
			rig := newIdentityRig(t, stage)
			arrange(rig)
			err := rig.o.verifyResumeIdentity(context.Background(), CycleRequest{}, 4)
			if (stage == config.StageEnforce) != (err != nil) {
				t.Errorf("%s at %s: err = %v", name, stage, err)
			}
			if stage == config.StageShadow && !strings.Contains(rig.stderr.String(), "not verified") {
				t.Errorf("%s: no WARN: %q", name, rig.stderr.String())
			}
		}
	}
}

func TestVerifyResumeIdentity_OverrideAppendFailureRefuses(t *testing.T) {
	t.Parallel()
	rig := newIdentityRig(t, config.StageEnforce)
	rig.start(t, 5)
	rig.now = cycleidentity.Identity{Graph: "g2", Contract: "c1", Policy: "p1"}
	rig.ledger.failOnAppend = true
	if err := rig.o.verifyResumeIdentity(context.Background(), CycleRequest{AcceptIdentityChange: true}, 5); err == nil {
		t.Fatal("a failed override record must refuse the resume")
	}
}

func TestRecordedIdentity_SkipsOtherCyclesAndKinds(t *testing.T) {
	t.Parallel()
	rig := newIdentityRig(t, config.StageShadow)
	rig.ledger.entries = []LedgerEntry{
		{Kind: "phase", Cycle: 8},
		{Kind: LedgerKindCycleIdentity, Cycle: 7, Message: cycleidentity.Encode(cycleidentity.Identity{Graph: "other"})},
		{Kind: LedgerKindCycleIdentity, Cycle: 8, Message: cycleidentity.Encode(idStart)},
	}
	got, err := rig.o.recordedIdentity(context.Background(), 8)
	if err != nil || got != idStart {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func resumeIdentityRig(t *testing.T, stage config.Stage) (*identityRig, *Orchestrator) {
	t.Helper()
	rig := &identityRig{ledger: &fakeLedger{}, stderr: &bytes.Buffer{}, center: signalcenter.New(), now: idStart}
	_, _, o := resumeRigE4(t, t.TempDir(), nil)
	o.ledger = rig.ledger
	WithStderr(rig.stderr)(o)
	WithSignalCenter(rig.center)(o)
	WithResumeIdentity(ResumeIdentityConfig{Stage: stage, Compute: func(string) (cycleidentity.Identity, error) { return rig.now, nil }})(o)
	rig.o = o
	rig.start(t, 61)
	return rig, o
}

func TestRunCycleFromPhase_RefusesAChangedGraphAtEnforceAndNamesIt(t *testing.T) {
	t.Parallel()
	rig, o := resumeIdentityRig(t, config.StageEnforce)
	rig.now = cycleidentity.Identity{Graph: "g2", Contract: "c1", Policy: "p1"}
	_, err := resumeFromE4(o, PhaseScout)
	var mm *cycleidentity.MismatchError
	if !errors.As(err, &mm) || len(mm.Parts) != 1 || mm.Parts[0] != cycleidentity.PartGraph {
		t.Fatalf("err = %v, want a MismatchError naming graph", err)
	}
}

func TestRunCycleFromPhase_ProceedsAtShadowWithTheSignalAndWithAMigrationRecord(t *testing.T) {
	t.Parallel()
	rig, o := resumeIdentityRig(t, config.StageShadow)
	rig.now = cycleidentity.Identity{Graph: "g2", Contract: "c1", Policy: "p1"}
	if _, err := resumeFromE4(o, PhaseScout); errors.Is(err, cycleidentity.ErrMismatch) {
		t.Fatalf("shadow must not refuse: %v", err)
	}
	if rig.shadowSignals() != 1 {
		t.Fatalf("shadow signals = %d, want 1", rig.shadowSignals())
	}
	rig2, o2 := resumeIdentityRig(t, config.StageEnforce)
	rig2.now = cycleidentity.Identity{Graph: "g2", Contract: "c1", Policy: "p1"}
	_, err := o2.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: "/nonexistent-e4-root", AcceptIdentityChange: true}, &ResumePoint{Phase: string(PhaseScout), CycleID: 61})
	if errors.Is(err, cycleidentity.ErrMismatch) {
		t.Fatalf("a migration must not refuse: %v", err)
	}
	overrides := 0
	for _, e := range rig2.ledger.entries {
		if e.Kind == LedgerKindCycleIdentityOverride {
			overrides++
		}
	}
	if overrides != 1 {
		t.Fatalf("override records = %d, want 1", overrides)
	}
}

func TestVerifyResume_UsesTheCheckpointCycleWhenTheStateHasNone(t *testing.T) {
	t.Parallel()
	rig := newIdentityRig(t, config.StageEnforce)
	rig.start(t, 12)
	rig.now = cycleidentity.Identity{Graph: "g1", Contract: "c1", Policy: "p2"}
	err := rig.o.verifyResume(context.Background(), resumeCheck{point: &ResumePoint{CycleID: 12}})
	var mm *cycleidentity.MismatchError
	if !errors.As(err, &mm) || mm.Parts[0] != cycleidentity.PartPolicy {
		t.Fatalf("err = %v, want the policy change of cycle 12", err)
	}
}

func TestVerifyResume_StopsOnAHealFailureBeforeTheIdentityCheck(t *testing.T) {
	t.Parallel()
	rig := newIdentityRig(t, config.StageEnforce)
	rig.start(t, 12)
	rig.now = cycleidentity.Identity{Graph: "g2", Contract: "c1", Policy: "p1"}
	err := rig.o.verifyResume(context.Background(), resumeCheck{point: &ResumePoint{CycleID: 12}, state: CycleState{CycleID: 13}})
	if err == nil || errors.Is(err, cycleidentity.ErrMismatch) {
		t.Fatalf("err = %v, want the cycle mismatch of the checkpoint", err)
	}
}

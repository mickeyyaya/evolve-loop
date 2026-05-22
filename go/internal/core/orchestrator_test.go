package core

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// Orchestrator phase-1 test surface — uses fake adapters to verify the
// sequencing contract without touching disk or processes. Real adapter
// impls land in Phase 2.

// --- fakes ---

type fakeStorage struct {
	state           State
	cycleState      CycleState
	cycleStateLog   []CycleState
	stateLog        []State
	lockHeld        bool
	lockCount       int
	mu              sync.Mutex
	lockErr         error
	failOnWriteCS   bool
}

func (f *fakeStorage) ReadState(_ context.Context) (State, error) {
	return f.state, nil
}
func (f *fakeStorage) WriteState(_ context.Context, s State) error {
	f.stateLog = append(f.stateLog, s)
	f.state = s
	return nil
}
func (f *fakeStorage) ReadCycleState(_ context.Context) (CycleState, error) {
	return f.cycleState, nil
}
func (f *fakeStorage) WriteCycleState(_ context.Context, cs CycleState) error {
	if f.failOnWriteCS {
		return errors.New("write CS forced fail")
	}
	f.cycleState = cs
	// Deep enough copy for the slice — the orchestrator may keep mutating.
	csCopy := cs
	csCopy.CompletedPhases = append([]string(nil), cs.CompletedPhases...)
	f.cycleStateLog = append(f.cycleStateLog, csCopy)
	return nil
}
func (f *fakeStorage) AcquireLock(_ context.Context) (func() error, error) {
	if f.lockErr != nil {
		return nil, f.lockErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lockHeld {
		return nil, ErrLockHeld
	}
	f.lockHeld = true
	f.lockCount++
	return func() error {
		f.mu.Lock()
		f.lockHeld = false
		f.mu.Unlock()
		return nil
	}, nil
}

type fakeLedger struct {
	entries []LedgerEntry
	mu      sync.Mutex
}

func (f *fakeLedger) Append(_ context.Context, e LedgerEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = append(f.entries, e)
	return nil
}
func (f *fakeLedger) Verify(_ context.Context) error { return nil }
func (f *fakeLedger) Iter(_ context.Context) (LedgerIterator, error) {
	return nil, errors.New("not used in tests")
}

// fakeRunner records every call. verdict[i] is the verdict returned on
// the i-th call; later calls return the last entry.
type fakeRunner struct {
	name     string
	calls    int
	requests []PhaseRequest
	verdict  string
}

func (f *fakeRunner) Name() string { return f.name }
func (f *fakeRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	f.calls++
	f.requests = append(f.requests, req)
	v := f.verdict
	if v == "" {
		v = VerdictPASS
	}
	return PhaseResponse{
		Phase:        f.name,
		Verdict:      v,
		ArtifactsDir: req.Workspace,
	}, nil
}

func buildRunners(verdicts map[Phase]string) map[Phase]PhaseRunner {
	out := map[Phase]PhaseRunner{}
	phases := []Phase{PhaseIntent, PhaseScout, PhaseTriage, PhaseTDD,
		PhaseBuild, PhaseAudit, PhaseShip, PhaseRetro}
	for _, p := range phases {
		out[p] = &fakeRunner{name: string(p), verdict: verdicts[p]}
	}
	return out
}

// --- tests ---

func TestOrchestrator_HappyPath_RunsAllPhasesInOrder(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 9}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners)

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: "/tmp/p",
		GoalHash:    "goal-1",
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if res.Cycle != 10 {
		t.Errorf("cycle=%d, want 10 (was 9, +1)", res.Cycle)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Errorf("verdict=%s, want PASS", res.FinalVerdict)
	}
	want := []Phase{PhaseScout, PhaseTriage, PhaseTDD, PhaseBuild, PhaseAudit, PhaseShip}
	if got := res.PhasesRun; len(got) != len(want) {
		t.Fatalf("phases=%v, want %v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("phase[%d]=%s, want %s", i, got[i], want[i])
			}
		}
	}
	// One ledger entry per phase that ran.
	if len(led.entries) != len(want) {
		t.Errorf("ledger entries=%d, want %d", len(led.entries), len(want))
	}
	for i, e := range led.entries {
		if e.Role != string(want[i]) {
			t.Errorf("ledger[%d].role=%s, want %s", i, e.Role, want[i])
		}
		if e.Cycle != 10 {
			t.Errorf("ledger[%d].cycle=%d, want 10", i, e.Cycle)
		}
	}
}

func TestOrchestrator_AuditFAIL_RoutesThroughRetro(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(map[Phase]string{PhaseAudit: VerdictFAIL})
	o := NewOrchestrator(st, led, runners)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	// Sequence should include retro after audit.
	foundRetro := false
	for _, p := range res.PhasesRun {
		if p == PhaseRetro {
			foundRetro = true
		}
	}
	if !foundRetro {
		t.Errorf("FAIL audit did not route through retro; ran %v", res.PhasesRun)
	}
}

func TestOrchestrator_AcquiresAndReleasesLock(t *testing.T) {
	st := &fakeStorage{}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if st.lockCount != 1 {
		t.Errorf("lockCount=%d, want 1", st.lockCount)
	}
	if st.lockHeld {
		t.Error("lock not released")
	}
}

func TestOrchestrator_LockErrorFailsFast(t *testing.T) {
	st := &fakeStorage{lockErr: ErrLockHeld}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"})
	if !errors.Is(err, ErrLockHeld) {
		t.Errorf("err=%v, want ErrLockHeld", err)
	}
	if len(led.entries) != 0 {
		t.Errorf("ledger written despite lock error: %d entries", len(led.entries))
	}
}

func TestOrchestrator_MissingRunnerErrors(t *testing.T) {
	st := &fakeStorage{}
	led := &fakeLedger{}
	runners := map[Phase]PhaseRunner{
		// missing scout
		PhaseTriage: &fakeRunner{name: "triage"},
	}
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"})
	if err == nil {
		t.Fatal("expected error for missing scout runner")
	}
}

func TestOrchestrator_AdvancesLastCycleNumber(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 41}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if st.state.LastCycleNumber != 42 {
		t.Errorf("lastCycleNumber=%d, want 42", st.state.LastCycleNumber)
	}
}

func TestOrchestrator_RecordsCompletedPhases(t *testing.T) {
	st := &fakeStorage{}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	// Final cycle-state should list every phase that ran.
	final := st.cycleState
	wantPhases := []string{"scout", "triage", "tdd", "build", "audit", "ship"}
	if len(final.CompletedPhases) != len(wantPhases) {
		t.Fatalf("completed=%v, want %v", final.CompletedPhases, wantPhases)
	}
	for i, p := range wantPhases {
		if final.CompletedPhases[i] != p {
			t.Errorf("completed[%d]=%s, want %s", i, final.CompletedPhases[i], p)
		}
	}
}

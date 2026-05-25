package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

// Orchestrator phase-1 test surface — uses fake adapters to verify the
// sequencing contract without touching disk or processes. Real adapter
// impls land in Phase 2.

// --- fakes ---

type fakeStorage struct {
	state            State
	cycleState       CycleState
	cycleStateLog    []CycleState
	stateLog         []State
	lockHeld         bool
	lockCount        int
	mu               sync.Mutex
	lockErr          error
	failOnWriteCS    bool
	failOnReadState  bool
	failOnWriteState bool
	writeCSFailAt    int // 0 = never; N = N-th write
	writeCSCalls     int
}

func (f *fakeStorage) ReadState(_ context.Context) (State, error) {
	if f.failOnReadState {
		return State{}, errors.New("forced ReadState fail")
	}
	return f.state, nil
}
func (f *fakeStorage) WriteState(_ context.Context, s State) error {
	if f.failOnWriteState {
		return errors.New("forced WriteState fail")
	}
	f.stateLog = append(f.stateLog, s)
	f.state = s
	return nil
}
func (f *fakeStorage) ReadCycleState(_ context.Context) (CycleState, error) {
	return f.cycleState, nil
}
func (f *fakeStorage) WriteCycleState(_ context.Context, cs CycleState) error {
	f.writeCSCalls++
	if f.failOnWriteCS {
		return errors.New("write CS forced fail")
	}
	if f.writeCSFailAt > 0 && f.writeCSCalls == f.writeCSFailAt {
		return errors.New("write CS forced fail at N")
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
	entries      []LedgerEntry
	mu           sync.Mutex
	failOnAppend bool
}

func (f *fakeLedger) Append(_ context.Context, e LedgerEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOnAppend {
		return errors.New("forced ledger append fail")
	}
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
		PhaseBuildPlanner, PhaseBuild, PhaseAudit, PhaseShip, PhaseRetro}
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
	want := []Phase{PhaseScout, PhaseTriage, PhaseTDD, PhaseBuildPlanner, PhaseBuild, PhaseAudit, PhaseShip}
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

// CycleRequest.Env must reach every PhaseRequest.Env. Phases consult
// req.Env["EVOLVE_CLI"] and req.Env["EVOLVE_*_MODEL"] for CLI/model
// selection; without this passthrough every cycle is silently hardcoded
// to claude-p + default model.
func TestOrchestrator_CycleEnv_PropagatesToEveryPhase(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners)

	envIn := map[string]string{
		"EVOLVE_CLI":         "codex",
		"EVOLVE_SCOUT_MODEL": "auto",
		"EVOLVE_BUILD_MODEL": "sonnet",
	}
	_, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: "/tmp/p",
		GoalHash:    "g",
		Env:         envIn,
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	// Every fakeRunner should have seen these env vars.
	for _, p := range []Phase{PhaseScout, PhaseTriage, PhaseTDD, PhaseBuildPlanner, PhaseBuild, PhaseAudit, PhaseShip} {
		fr := runners[p].(*fakeRunner)
		if fr.calls == 0 {
			t.Errorf("phase %s never ran", p)
			continue
		}
		got := fr.requests[0].Env
		if got["EVOLVE_CLI"] != "codex" {
			t.Errorf("phase %s: req.Env[EVOLVE_CLI]=%q, want codex", p, got["EVOLVE_CLI"])
		}
		if got["EVOLVE_BUILD_MODEL"] != "sonnet" {
			t.Errorf("phase %s: req.Env[EVOLVE_BUILD_MODEL]=%q, want sonnet", p, got["EVOLVE_BUILD_MODEL"])
		}
	}
}

// Mutating the operator's Env map post-RunCycle must not retroactively
// change what phases saw — the orchestrator must copy the map.
func TestOrchestrator_CycleEnv_IsCopied(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners)

	envIn := map[string]string{"EVOLVE_CLI": "codex"}
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p", Env: envIn})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	envIn["EVOLVE_CLI"] = "MUTATED"
	for _, p := range []Phase{PhaseScout, PhaseBuild} {
		fr := runners[p].(*fakeRunner)
		if got := fr.requests[0].Env["EVOLVE_CLI"]; got != "codex" {
			t.Errorf("phase %s: req.Env[EVOLVE_CLI]=%q, want codex (operator mutation must not propagate)", p, got)
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

func TestOrchestrator_ReadStateError(t *testing.T) {
	st := &fakeStorage{failOnReadState: true}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"})
	if err == nil {
		t.Fatal("ReadState error must propagate")
	}
}

func TestOrchestrator_InitialWriteCycleStateError(t *testing.T) {
	st := &fakeStorage{failOnWriteCS: true}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"})
	if err == nil {
		t.Fatal("initial WriteCycleState error must propagate")
	}
}

func TestOrchestrator_WriteCycleStateMidPhaseError(t *testing.T) {
	// Fail on the 2nd write (after init). The orchestrator writes
	// pre-phase and post-phase, so this fails before scout's run.
	st := &fakeStorage{writeCSFailAt: 2}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"})
	if err == nil {
		t.Fatal("mid-phase WriteCycleState error must propagate")
	}
}

func TestOrchestrator_WriteCycleStatePostPhaseError(t *testing.T) {
	// Init=1, pre-scout=2, post-scout=3 → fail at 3.
	st := &fakeStorage{writeCSFailAt: 3}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"})
	if err == nil {
		t.Fatal("post-phase WriteCycleState error must propagate")
	}
}

func TestOrchestrator_LedgerAppendError(t *testing.T) {
	st := &fakeStorage{}
	led := &fakeLedger{failOnAppend: true}
	o := NewOrchestrator(st, led, buildRunners(nil))
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"})
	if err == nil {
		t.Fatal("ledger append error must propagate")
	}
}

func TestOrchestrator_FinalWriteStateError(t *testing.T) {
	st := &fakeStorage{failOnWriteState: true}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"})
	if err == nil {
		t.Fatal("final WriteState error must propagate")
	}
}

// A runner that returns an error from Run.
type erroringRunner struct{ name string }

func (e *erroringRunner) Name() string { return e.name }
func (e *erroringRunner) Run(context.Context, PhaseRequest) (PhaseResponse, error) {
	return PhaseResponse{}, errors.New("runner forced fail")
}

func TestOrchestrator_RunnerErrorPropagates(t *testing.T) {
	st := &fakeStorage{}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseScout] = &erroringRunner{name: "scout"}
	o := NewOrchestrator(st, led, runners)
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"})
	if err == nil {
		t.Fatal("runner error must propagate")
	}
}

// A runner that returns a non-canonical verdict.
type badVerdictRunner struct{ name string }

func (b *badVerdictRunner) Name() string { return b.name }
func (b *badVerdictRunner) Run(context.Context, PhaseRequest) (PhaseResponse, error) {
	return PhaseResponse{Phase: b.name, Verdict: "bogus"}, nil
}

func TestOrchestrator_NonCanonicalVerdictRejected(t *testing.T) {
	st := &fakeStorage{}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseScout] = &badVerdictRunner{name: "scout"}
	o := NewOrchestrator(st, led, runners)
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"})
	if err == nil {
		t.Fatal("non-canonical verdict must be rejected")
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
	wantPhases := []string{"scout", "triage", "tdd", "build-planner", "build", "audit", "ship"}
	if len(final.CompletedPhases) != len(wantPhases) {
		t.Fatalf("completed=%v, want %v", final.CompletedPhases, wantPhases)
	}
	for i, p := range wantPhases {
		if final.CompletedPhases[i] != p {
			t.Errorf("completed[%d]=%s, want %s", i, final.CompletedPhases[i], p)
		}
	}
}

// --- intent-gate tests (M2 wiring) ---

// When intent is not required, the first phase to run is Scout — the
// historical default. Verified by the happy-path test above; this one
// just asserts that PhaseIntent did NOT execute and that the runner
// registered for intent was never invoked.
func TestOrchestrator_IntentGate_DefaultRunsScoutFirst(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	intent := runners[PhaseIntent].(*fakeRunner)
	if intent.calls != 0 {
		t.Errorf("intent ran %d times; expected 0 when intent_required=false", intent.calls)
	}
	if len(res.PhasesRun) == 0 || res.PhasesRun[0] != PhaseScout {
		t.Errorf("phases[0]=%v, want scout", res.PhasesRun)
	}
	// CycleState should record intent_required=false for downstream
	// consumers (resume / classifier).
	if st.cycleState.IntentRequired {
		t.Errorf("CycleState.IntentRequired=true, want false")
	}
}

// EVOLVE_REQUIRE_INTENT=1 in CycleRequest.Env triggers the intent phase
// before Scout. CycleState.IntentRequired is persisted so resume +
// downstream consumers can read it.
func TestOrchestrator_IntentGate_EnvVarRunsIntentFirst(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners)

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: "/tmp/p",
		Env:         map[string]string{"EVOLVE_REQUIRE_INTENT": "1"},
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	intent := runners[PhaseIntent].(*fakeRunner)
	if intent.calls != 1 {
		t.Fatalf("intent ran %d times; expected 1", intent.calls)
	}
	want := []Phase{PhaseIntent, PhaseScout, PhaseTriage, PhaseTDD, PhaseBuildPlanner, PhaseBuild, PhaseAudit, PhaseShip}
	if len(res.PhasesRun) != len(want) {
		t.Fatalf("phases=%v, want %v", res.PhasesRun, want)
	}
	for i, p := range want {
		if res.PhasesRun[i] != p {
			t.Errorf("phase[%d]=%s, want %s", i, res.PhasesRun[i], p)
		}
	}
	if !st.cycleState.IntentRequired {
		t.Errorf("CycleState.IntentRequired=false, want true")
	}
}

// Context["intent_required"]="true" is the explicit caller-side knob;
// it should also trigger intent regardless of env. Source priority is
// Context > Env in the orchestrator.
func TestOrchestrator_IntentGate_ContextOverrideRunsIntent(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: "/tmp/p",
		Context:     map[string]string{"intent_required": "true"},
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	intent := runners[PhaseIntent].(*fakeRunner)
	if intent.calls != 1 {
		t.Errorf("intent ran %d times; expected 1 from Context override", intent.calls)
	}
}

func TestStateMachine_NextFromStart(t *testing.T) {
	sm := NewStateMachine()
	if got := sm.NextFromStart(false); got != PhaseScout {
		t.Errorf("NextFromStart(false)=%s, want scout", got)
	}
	if got := sm.NextFromStart(true); got != PhaseIntent {
		t.Errorf("NextFromStart(true)=%s, want intent", got)
	}
}

// --- failure-adapter retro branching (M3) ---

// Retro PASS short-circuits to ship — failureadapter not consulted.
func TestOrchestrator_RetroPASS_RoutesToShip(t *testing.T) {
	st := &fakeStorage{state: State{
		LastCycleNumber: 0,
		FailedAt: []FailedRecord{
			// Even with prior failures, retro PASS overrides and ships.
			{Cycle: 1, Verdict: "FAIL", Classification: "code-build-fail"},
			{Cycle: 2, Verdict: "FAIL", Classification: "code-build-fail"},
		},
	}}
	led := &fakeLedger{}
	runners := buildRunners(map[Phase]string{
		PhaseAudit: VerdictFAIL, // force audit→retro path
		PhaseRetro: VerdictPASS,
	})
	o := NewOrchestrator(st, led, runners)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	// After retro PASS, ship should have run.
	wantTail := []Phase{PhaseAudit, PhaseRetro, PhaseShip}
	got := res.PhasesRun
	if len(got) < len(wantTail) {
		t.Fatalf("not enough phases: %v", got)
	}
	tail := got[len(got)-len(wantTail):]
	for i, p := range wantTail {
		if tail[i] != p {
			t.Errorf("tail[%d]=%s, want %s; full=%v", i, tail[i], p, got)
		}
	}
	if res.RetroDecision != "retro-recovered: ship" {
		t.Errorf("RetroDecision=%q, want retro-recovered: ship", res.RetroDecision)
	}
}

// Retro FAIL + clean failedApproaches → PROCEED → end (no ship, no retry).
func TestOrchestrator_RetroFAIL_NoHistory_RoutesToEnd(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(map[Phase]string{
		PhaseAudit: VerdictFAIL,
		PhaseRetro: VerdictFAIL,
	})
	o := NewOrchestrator(st, led, runners)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	// Retro is the last phase — no ship, no second tdd.
	last := res.PhasesRun[len(res.PhasesRun)-1]
	if last != PhaseRetro {
		t.Errorf("last phase=%s, want retro", last)
	}
	if !strings.HasPrefix(res.RetroDecision, "proceed:") {
		t.Errorf("RetroDecision=%q, want proceed: prefix", res.RetroDecision)
	}
}

// Retro FAIL + 2 distinct code-audit-fail records → BLOCK-CODE (strict)
// or PROCEED awareness (fluent default). Default fluent mode → end.
func TestOrchestrator_RetroFAIL_RecurringAudit_FluentEnd(t *testing.T) {
	st := &fakeStorage{state: State{
		LastCycleNumber: 5,
		FailedAt: []FailedRecord{
			{Cycle: 3, Verdict: "FAIL", Classification: "code-audit-fail", RecordedAt: "2099-01-01T00:00:00Z"},
			{Cycle: 4, Verdict: "FAIL", Classification: "code-audit-fail", RecordedAt: "2099-01-01T00:00:00Z"},
		},
	}}
	led := &fakeLedger{}
	runners := buildRunners(map[Phase]string{
		PhaseAudit: VerdictFAIL,
		PhaseRetro: VerdictFAIL,
	})
	o := NewOrchestrator(st, led, runners)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	// Fluent mode default → PROCEED with awareness → end.
	last := res.PhasesRun[len(res.PhasesRun)-1]
	if last != PhaseRetro {
		t.Errorf("last phase=%s, want retro (fluent proceed→end)", last)
	}
	if !strings.HasPrefix(res.RetroDecision, "proceed:") {
		t.Errorf("RetroDecision=%q, want proceed: prefix", res.RetroDecision)
	}
}

// entriesFromRecords sanity: classification + retrospected fields survive
// the cross-package projection.
func TestEntriesFromRecords_PreservesClassification(t *testing.T) {
	records := []FailedRecord{
		{Cycle: 1, Verdict: "FAIL", Classification: "code-build-fail", Retrospected: true},
		{Cycle: 2, Verdict: "FAIL", Classification: "infrastructure-transient", RecordedAt: "2026-05-23T00:00:00Z"},
	}
	entries := entriesFromRecords(records)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if string(entries[0].Classification) != "code-build-fail" {
		t.Errorf("entries[0].Classification=%q", entries[0].Classification)
	}
	if !entries[0].Retrospected {
		t.Errorf("retrospected lost")
	}
	if entries[1].RecordedAt != "2026-05-23T00:00:00Z" {
		t.Errorf("recordedAt lost")
	}
}

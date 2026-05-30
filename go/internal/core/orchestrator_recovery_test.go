package core_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// --- minimal test harness for core_test package ---

// recStorage is a minimal in-memory Storage for recovery tests.
type recStorage struct {
	mu    sync.Mutex
	state core.State
	cs    core.CycleState
}

func (s *recStorage) ReadState(_ context.Context) (core.State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state, nil
}
func (s *recStorage) WriteState(_ context.Context, st core.State) error {
	s.mu.Lock()
	s.state = st
	s.mu.Unlock()
	return nil
}
func (s *recStorage) ReadCycleState(_ context.Context) (core.CycleState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cs, nil
}
func (s *recStorage) WriteCycleState(_ context.Context, cs core.CycleState) error {
	s.mu.Lock()
	s.cs = cs
	s.mu.Unlock()
	return nil
}
func (s *recStorage) AcquireLock(_ context.Context) (func() error, error) {
	return func() error { return nil }, nil
}

// fakeLedger is the core_test-package ledger stub.
type fakeLedger struct {
	mu      sync.Mutex
	entries []core.LedgerEntry
}

func (l *fakeLedger) Append(_ context.Context, e core.LedgerEntry) error {
	l.mu.Lock()
	l.entries = append(l.entries, e)
	l.mu.Unlock()
	return nil
}
func (l *fakeLedger) Verify(_ context.Context) error { return nil }
func (l *fakeLedger) Iter(_ context.Context) (core.LedgerIterator, error) {
	return nil, errors.New("not implemented")
}
func (l *fakeLedger) entryKinds() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	kinds := make([]string, len(l.entries))
	for i, e := range l.entries {
		kinds[i] = e.Kind
	}
	return kinds
}

// allPhases is the ordered set of phases a complete runner map must cover.
var allPhases = []core.Phase{
	core.PhaseIntent, core.PhaseScout, core.PhaseTriage, core.PhaseTDD,
	core.PhaseBuildPlanner, core.PhaseBuild, core.PhaseAudit,
	core.PhaseShip, core.PhaseRetro, core.PhaseDebugger,
}

// passRunner is a PhaseRunner that always returns PASS.
type passRunner struct{ name string }

func (r *passRunner) Name() string { return r.name }
func (r *passRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{Phase: r.name, Verdict: core.VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

// newRunners builds a full phase→runner map, replacing phases present in over.
func newRunners(over map[core.Phase]core.PhaseRunner) map[core.Phase]core.PhaseRunner {
	out := make(map[core.Phase]core.PhaseRunner, len(allPhases))
	for _, p := range allPhases {
		out[p] = &passRunner{name: string(p)}
	}
	for p, r := range over {
		out[p] = r
	}
	return out
}

// newTestOrchestrator constructs an Orchestrator with in-memory fakes.
// Returns (orchestrator, storage, ledger) — storage is rarely needed by callers
// but is returned for completeness (callers discard it with _).
func newTestOrchestrator(t *testing.T, runners map[core.Phase]core.PhaseRunner) (*core.Orchestrator, *recStorage, *fakeLedger) {
	t.Helper()
	st := &recStorage{}
	ld := &fakeLedger{}
	return core.NewOrchestrator(st, ld, runners), st, ld
}

// runCycleT runs a cycle with a minimal CycleRequest and returns the result.
func runCycleT(t *testing.T, o *core.Orchestrator) (core.CycleResult, error) {
	t.Helper()
	return o.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: t.TempDir(),
		GoalHash:    "test-goal",
		Context:     map[string]string{"commit_message": "test commit"},
	})
}

// shipErrorStub is a PhaseRunner that returns a ShipError on its first N calls
// then a PASS, to exercise the orchestrator's ship-error recovery seam
// (Component #7). It records how many times Run was invoked.
type shipErrorStub struct {
	name      string
	failFirst int // number of leading calls that return errOnFail
	errOnFail error
	calls     int
}

func (s *shipErrorStub) Name() string { return s.name }

func (s *shipErrorStub) Run(_ context.Context, _ core.PhaseRequest) (core.PhaseResponse, error) {
	s.calls++
	if s.calls <= s.failFirst {
		return core.PhaseResponse{Phase: s.name, Verdict: core.VerdictFAIL}, s.errOnFail
	}
	return core.PhaseResponse{Phase: s.name, Verdict: core.VerdictPASS}, nil
}

// signalStub is a PhaseRunner returning a fixed PASS response with the given
// Signals — used to fake the debugger phase's recovery decision.
type signalStub struct {
	name    string
	signals map[string]any
	calls   int
}

func (s *signalStub) Name() string { return s.name }

func (s *signalStub) Run(_ context.Context, _ core.PhaseRequest) (core.PhaseResponse, error) {
	s.calls++
	return core.PhaseResponse{Phase: s.name, Verdict: core.VerdictPASS, Signals: s.signals}, nil
}

func runRecoveryCycle(t *testing.T, over map[core.Phase]core.PhaseRunner) (core.CycleResult, error, *fakeLedger) {
	t.Helper()
	orch, _, ld := newTestOrchestrator(t, newRunners(over))
	res, err := runCycleT(t, orch)
	return res, err, ld
}

// Precondition-class ShipError (e.g. AUDIT_BINDING_HEAD_MOVED) → the recovery
// chain re-runs audit (saga alternative path); ship then succeeds. The cycle
// completes without error and ship is invoked twice.
func TestRunCycle_ShipPreconditionError_ReRunsAuditThenShips(t *testing.T) {
	ship := &shipErrorStub{
		name:      "ship",
		failFirst: 1,
		errOnFail: core.NewShipError(core.CodeAuditBindingHeadMoved, core.ShipClassPrecondition, core.StageVerifyClass, "stale binding"),
	}
	res, err, ld := runRecoveryCycle(t, map[core.Phase]core.PhaseRunner{core.PhaseShip: ship})
	if err != nil {
		t.Fatalf("recoverable precondition ship error should NOT abort the cycle, got: %v", err)
	}
	if ship.calls != 2 {
		t.Fatalf("ship calls = %d, want 2 (fail → re-audit → ship succeeds)", ship.calls)
	}
	if !containsKind(ld, "ship_error") {
		t.Fatalf("expected a ship_error ledger entry; kinds=%v", ld.entryKinds())
	}
	_ = res
}

// Transient-class ShipError (e.g. GIT_PUSH_REJECTED) → retry ship directly.
func TestRunCycle_ShipTransientError_RetriesShip(t *testing.T) {
	ship := &shipErrorStub{
		name:      "ship",
		failFirst: 1,
		errOnFail: core.NewShipError(core.CodeGitPushRejected, core.ShipClassTransient, core.StageAtomicShip, "push race"),
	}
	_, err, _ := runRecoveryCycle(t, map[core.Phase]core.PhaseRunner{core.PhaseShip: ship})
	if err != nil {
		t.Fatalf("transient ship error should retry+succeed, got: %v", err)
	}
	if ship.calls != 2 {
		t.Fatalf("ship calls = %d, want 2 (fail → retry ship)", ship.calls)
	}
}

// Integrity-class ShipError (e.g. INTEGRITY_TREE_DRIFT) → BLOCK: the cycle
// aborts loudly with the ShipError surfaced, never auto-recovered.
func TestRunCycle_ShipIntegrityError_AbortsLoud(t *testing.T) {
	se := core.NewShipError(core.CodeIntegrityTreeDrift, core.ShipClassIntegrity, core.StageAtomicShip, "tree drift")
	ship := &shipErrorStub{name: "ship", failFirst: 99, errOnFail: se}
	_, err, _ := runRecoveryCycle(t, map[core.Phase]core.PhaseRunner{core.PhaseShip: ship})
	if err == nil {
		t.Fatal("integrity ship error MUST abort the cycle")
	}
	if got, ok := core.AsShipError(err); !ok || got.Code != core.CodeIntegrityTreeDrift {
		t.Fatalf("aborted error should carry the integrity ShipError, got: %v", err)
	}
	if ship.calls != 1 {
		t.Fatalf("ship calls = %d, want 1 (integrity never retries)", ship.calls)
	}
}

// A persistently-failing precondition error exhausts maxRecoveryDepth (2) and
// then aborts — the bounded-recursion safety invariant.
func TestRunCycle_ShipRecoveryExhausted_Aborts(t *testing.T) {
	se := core.NewShipError(core.CodeAuditBindingHeadMoved, core.ShipClassPrecondition, core.StageVerifyClass, "always stale")
	ship := &shipErrorStub{name: "ship", failFirst: 99, errOnFail: se}
	_, err, _ := runRecoveryCycle(t, map[core.Phase]core.PhaseRunner{core.PhaseShip: ship})
	if err == nil {
		t.Fatal("a never-resolving ship error must abort once recovery depth is exhausted")
	}
	// depth 0 (initial) + 2 recoveries = 3 ship attempts, then abort.
	if ship.calls != 3 {
		t.Fatalf("ship calls = %d, want 3 (initial + maxRecoveryDepth=2)", ship.calls)
	}
}

// Unknown/unmapped ShipError → debugger phase; its RESHIP decision routes back
// to ship, which then succeeds.
func TestRunCycle_ShipUnknownError_DebuggerReship(t *testing.T) {
	se := core.NewShipError(core.CodeWorktreeResolve, core.ShipClassConfig, core.StageArgs, "novel")
	ship := &shipErrorStub{name: "ship", failFirst: 1, errOnFail: se}
	dbg := &signalStub{name: "debugger", signals: map[string]any{"debugger.action": "RESHIP"}}
	res, err, ld := runRecoveryCycle(t, map[core.Phase]core.PhaseRunner{
		core.PhaseShip:     ship,
		core.PhaseDebugger: dbg,
	})
	if err != nil {
		t.Fatalf("debugger RESHIP path should recover, got: %v", err)
	}
	if dbg.calls != 1 {
		t.Fatalf("debugger calls = %d, want 1", dbg.calls)
	}
	if ship.calls != 2 {
		t.Fatalf("ship calls = %d, want 2 (fail → debugger → reship)", ship.calls)
	}
	if !containsKind(ld, "debugger_decision") {
		t.Fatalf("expected a debugger_decision ledger entry; kinds=%v", ld.entryKinds())
	}
	_ = res
}

// Debugger RERUN_PHASE:audit → re-run audit, then ship succeeds.
func TestRunCycle_ShipUnknownError_DebuggerRerunAudit(t *testing.T) {
	se := core.NewShipError(core.CodeWorktreeResolve, core.ShipClassConfig, core.StageArgs, "novel")
	ship := &shipErrorStub{name: "ship", failFirst: 1, errOnFail: se}
	dbg := &signalStub{name: "debugger", signals: map[string]any{
		"debugger.action":      "RERUN_PHASE",
		"debugger.rerun_phase": "audit",
	}}
	_, err, _ := runRecoveryCycle(t, map[core.Phase]core.PhaseRunner{
		core.PhaseShip:     ship,
		core.PhaseDebugger: dbg,
	})
	if err != nil {
		t.Fatalf("debugger RERUN_PHASE:audit path should recover, got: %v", err)
	}
	if ship.calls != 2 {
		t.Fatalf("ship calls = %d, want 2 (fail → debugger → re-audit → ship)", ship.calls)
	}
}

// Debugger BLOCK → cycle ends without re-shipping (no further ship attempt).
func TestRunCycle_ShipUnknownError_DebuggerBlock(t *testing.T) {
	se := core.NewShipError(core.CodeWorktreeResolve, core.ShipClassConfig, core.StageArgs, "novel")
	ship := &shipErrorStub{name: "ship", failFirst: 99, errOnFail: se}
	dbg := &signalStub{name: "debugger", signals: map[string]any{"debugger.action": "BLOCK"}}
	_, err, _ := runRecoveryCycle(t, map[core.Phase]core.PhaseRunner{
		core.PhaseShip:     ship,
		core.PhaseDebugger: dbg,
	})
	if err != nil {
		t.Fatalf("debugger BLOCK ends the cycle cleanly (no abort error), got: %v", err)
	}
	if dbg.calls != 1 {
		t.Fatalf("debugger calls = %d, want 1", dbg.calls)
	}
	if ship.calls != 1 {
		t.Fatalf("ship calls = %d, want 1 (BLOCK → no reship)", ship.calls)
	}
}

// containsKind reports whether the ledger recorded an entry of the given kind.
func containsKind(ld *fakeLedger, kind string) bool {
	for _, k := range ld.entryKinds() {
		if k == kind {
			return true
		}
	}
	return false
}

// guard: keep the errors+strings imports meaningful if assertions evolve.
var _ = errors.Is
var _ = strings.Contains

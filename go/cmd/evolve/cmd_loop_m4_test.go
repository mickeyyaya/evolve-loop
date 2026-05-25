package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// fakeStorage is an in-memory core.Storage for runLoop integration
// tests. Mirrors the on-disk layout enough for cmd_loop helpers
// (readLastCycleNumber, LoadVerifyContext) to read realistic values.
type fakeStorage struct {
	mu     sync.Mutex
	state  core.State
	cs     core.CycleState
	lockFn func() error
}

func (s *fakeStorage) ReadState(context.Context) (core.State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state, nil
}
func (s *fakeStorage) WriteState(_ context.Context, st core.State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = st
	return nil
}
func (s *fakeStorage) ReadCycleState(context.Context) (core.CycleState, error) {
	return s.cs, nil
}
func (s *fakeStorage) WriteCycleState(_ context.Context, cs core.CycleState) error {
	s.cs = cs
	return nil
}
func (s *fakeStorage) AcquireLock(context.Context) (func() error, error) {
	if s.lockFn != nil {
		return s.lockFn, nil
	}
	return func() error { return nil }, nil
}

// fakeLedger answers Iter with a pre-baked slice; tests build the
// slice in advance to drive the verify pipeline through specific
// branches (complete cycle, missing builder, etc.).
type fakeLedger struct {
	mu      sync.Mutex
	entries []core.LedgerEntry
}

func (l *fakeLedger) Append(_ context.Context, e core.LedgerEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, e)
	return nil
}
func (l *fakeLedger) Verify(context.Context) error { return nil }
func (l *fakeLedger) Iter(context.Context) (core.LedgerIterator, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	cp := append([]core.LedgerEntry{}, l.entries...)
	return &sliceIter{entries: cp}, nil
}

type sliceIter struct {
	entries []core.LedgerEntry
	i       int
}

func (it *sliceIter) Next() (core.LedgerEntry, bool, error) {
	if it.i >= len(it.entries) {
		return core.LedgerEntry{}, false, nil
	}
	e := it.entries[it.i]
	it.i++
	return e, true, nil
}
func (it *sliceIter) Close() error { return nil }

// scriptedOrch is a *core.Orchestrator stand-in that returns canned
// cycle results in sequence. The real Orchestrator type is a struct,
// not an interface, so the test seam replaces wireOrchestratorDepsFn
// entirely — see helperStubDeps.
type scriptedOrch struct {
	results []core.CycleResult
	errs    []error
	storage *fakeStorage
	ledger  *fakeLedger
	idx     int
}

// noopRunner satisfies core.PhaseRunner by returning PASS for every
// phase without touching disk or invoking a CLI. Used by installStubDeps
// to let the orchestrator's state machine traverse start→end while
// the test controls the ledger contents and the per-cycle workspace.
type noopRunner struct{ name string }

func (n noopRunner) Name() string { return n.name }
func (noopRunner) Run(_ context.Context, _ core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{Verdict: core.VerdictPASS}, nil
}

// installStubDeps swaps wireOrchestratorDepsFn for one that returns
// the given fake storage + ledger backed by a noop-PASS orchestrator.
// The orchestrator's state machine runs but every phase is a no-op,
// so the only ledger entries are the phase-kind appends the
// orchestrator writes itself — verify will fail unless the test
// pre-seeds agent_subprocess entries via fakeLedger.entries.
func installStubDeps(t *testing.T, storage core.Storage, ledger core.Ledger) func() {
	t.Helper()
	prev := wireOrchestratorDepsFn
	wireOrchestratorDepsFn = func(string, string) orchDeps {
		runners := map[core.Phase]core.PhaseRunner{
			core.PhaseIntent: noopRunner{name: "intent"},
			core.PhaseScout:  noopRunner{name: "scout"},
			core.PhaseTriage: noopRunner{name: "triage"},
			core.PhaseTDD:    noopRunner{name: "tdd"},
			core.PhaseBuild:  noopRunner{name: "build"},
			core.PhaseAudit:  noopRunner{name: "audit"},
			core.PhaseShip:   noopRunner{name: "ship"},
			core.PhaseRetro:  noopRunner{name: "retro"},
		}
		return orchDeps{
			Storage:      storage,
			Ledger:       ledger,
			Orchestrator: core.NewOrchestrator(storage, ledger, runners),
		}
	}
	return func() { wireOrchestratorDepsFn = prev }
}

func TestResolveDispatchPolicy(t *testing.T) {
	// No t.Parallel: subtests mutate process env via t.Setenv.
	tests := []struct {
		name string
		env  map[string]string
		want dispatchPolicy
	}{
		{"default → verify", nil, dispatchPolicyVerify},
		{"explicit verify", map[string]string{"EVOLVE_DISPATCH_POLICY": "verify"}, dispatchPolicyVerify},
		{"explicit off", map[string]string{"EVOLVE_DISPATCH_POLICY": "off"}, dispatchPolicyOff},
		{"explicit stop", map[string]string{"EVOLVE_DISPATCH_POLICY": "stop"}, dispatchPolicyStop},
		{"unknown → verify (fallback)", map[string]string{"EVOLVE_DISPATCH_POLICY": "bogus"}, dispatchPolicyVerify},
		{"legacy STOP_ON_FAIL=1 bridges to stop", map[string]string{"EVOLVE_DISPATCH_STOP_ON_FAIL": "1"}, dispatchPolicyStop},
		{"legacy VERIFY=0 bridges to off", map[string]string{"EVOLVE_DISPATCH_VERIFY": "0"}, dispatchPolicyOff},
		{"both legacy + new — new wins", map[string]string{"EVOLVE_DISPATCH_POLICY": "verify", "EVOLVE_DISPATCH_STOP_ON_FAIL": "1"}, dispatchPolicyVerify},
		{"both legacy → STOP_ON_FAIL wins (most restrictive)", map[string]string{"EVOLVE_DISPATCH_STOP_ON_FAIL": "1", "EVOLVE_DISPATCH_VERIFY": "0"}, dispatchPolicyStop},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Not t.Parallel — uses process env. Use t.Setenv for isolation.
			for k := range envKeys {
				t.Setenv(k, "")
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			var stderr bytes.Buffer
			got := resolveDispatchPolicy(&stderr)
			if got != tc.want {
				t.Fatalf("policy=%v want %v (stderr=%q)", got, tc.want, stderr.String())
			}
		})
	}
}

// envKeys is the set t.Setenv-clears for policy tests. Centralized so
// adding a new env var only requires one update.
var envKeys = map[string]struct{}{
	"EVOLVE_DISPATCH_POLICY":       {},
	"EVOLVE_DISPATCH_STOP_ON_FAIL": {},
	"EVOLVE_DISPATCH_VERIFY":       {},
}

func TestResolveCircuitBreakerThreshold(t *testing.T) {
	// No t.Parallel: subtests mutate process env via t.Setenv.
	tests := []struct {
		val  string
		want int
	}{
		{"", defaultCircuitBreakerThreshold},
		{"3", 3},
		{"100", 100},
		{"0", defaultCircuitBreakerThreshold},
		{"-5", defaultCircuitBreakerThreshold},
		{"abc", defaultCircuitBreakerThreshold},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.val, func(t *testing.T) {
			t.Setenv("EVOLVE_DISPATCH_REPEAT_THRESHOLD", tc.val)
			if got := resolveCircuitBreakerThreshold(); got != tc.want {
				t.Fatalf("threshold(%q)=%d want %d", tc.val, got, tc.want)
			}
		})
	}
}

func TestDirExists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if !dirExists(dir) {
		t.Fatalf("dir %q should exist", dir)
	}
	if dirExists(filepath.Join(dir, "nope")) {
		t.Fatalf("nonexistent path returned true")
	}
	// File at the same name → false
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if dirExists(file) {
		t.Fatalf("file (not dir) returned true")
	}
}

func TestCycleWorkspace(t *testing.T) {
	t.Parallel()
	got := cycleWorkspace("/p", 7)
	want := filepath.Join("/p", ".evolve", "runs", "cycle-7")
	if got != want {
		t.Fatalf("workspace=%q want %q", got, want)
	}
}

func TestReadLastCycleNumber(t *testing.T) {
	t.Parallel()
	s := &fakeStorage{state: core.State{LastCycleNumber: 42}}
	n, err := readLastCycleNumber(context.Background(), s)
	if err != nil || n != 42 {
		t.Fatalf("got n=%d err=%v", n, err)
	}
}

// helperPrepWorkspace seeds a cycle workspace with the minimum files
// the post-cycle pipeline reads: orchestrator-report.md (drives
// classification) and optionally .cycle-verdict (drives memo gate).
func helperPrepWorkspace(t *testing.T, projectRoot string, cycle int, report, verdict string) string {
	t.Helper()
	ws := cycleWorkspace(projectRoot, cycle)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte(report), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	if verdict != "" {
		if err := os.WriteFile(filepath.Join(ws, ".cycle-verdict"), []byte(verdict), 0o644); err != nil {
			t.Fatalf("write verdict: %v", err)
		}
	}
	return ws
}

// Below tests exercise the M4 dispatcher pipeline end-to-end using
// installStubDeps. The stub orchestrator returns canned CycleResults
// so the post-cycle integration (verify+classify+events+breaker) runs
// against a fake ledger and a fake state writer.
//
// Each test wires its own custom Orchestrator-like flow by stubbing
// wireOrchestratorDepsFn AND populating the storage+ledger with the
// state that the orchestrator would have produced.

// runM4Loop prepares a stub that, on RunCycle, populates the fake
// ledger with the entries the test expects + advances state.
func runM4Loop(t *testing.T, projectRoot, evolveDir string, args []string,
	storage *fakeStorage, ledger *fakeLedger,
	beforeRun func(*fakeStorage, *fakeLedger),
	report, verdict string,
	cycleNum int,
) (int, string, string) {
	t.Helper()
	defer installStubDeps(t, storage, ledger)()

	// Override the Orchestrator.RunCycle by intercepting via a custom
	// wireOrchestratorDepsFn that returns a fake-driven *Orchestrator.
	// The real orchestrator runs a phase-machine — we don't want that.
	// Instead, set up so the orchestrator finishes the cycle quickly
	// by having NO runners registered (it'll return immediately with
	// the PhaseStart → PhaseEnd noop since intent_required=false and
	// scout is missing).
	//
	// Pre-populate side-effects the orchestrator would have made: the
	// ledger entries, the workspace, the state advance.
	helperPrepWorkspace(t, projectRoot, cycleNum, report, verdict)
	if beforeRun != nil {
		beforeRun(storage, ledger)
	}

	var stdout, stderr bytes.Buffer
	rc := runLoop(args, nil, &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

// TestRunLoop_PolicyOff_SkipsVerify verifies that EVOLVE_DISPATCH_POLICY=off
// does not run the verify pipeline. The fake ledger has zero entries
// (would normally trip "missing scout"), but policy=off skips the
// check entirely, so the loop exits cleanly.
func TestRunLoop_PolicyOff_SkipsVerify(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_POLICY", "off")

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	storage := &fakeStorage{}
	ledger := &fakeLedger{}

	args := []string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "test goal",
		"--cycles", "1",
	}
	// No workspace prep, no ledger entries — verify would fail in any
	// other mode but off skips entirely.
	rc, _, stderr := runM4Loop(t, projectRoot, evolveDir, args, storage, ledger, nil, "", "", 1)

	// Orchestrator with no runners completes PhaseStart→PhaseEnd
	// immediately. Loop exits cleanly with rc=0.
	if rc != 0 {
		t.Fatalf("rc=%d want 0; stderr=%q", rc, stderr)
	}
}

// TestRunLoop_PolicyVerify_RecoverableContinues seeds a workspace with
// a build-fail orchestrator-report and a ledger missing the auditor
// entry. policy=verify must classify build-fail and continue the loop
// for the next cycle (exit 0 because cycles=1 means batch ends).
func TestRunLoop_PolicyVerify_RecoverableContinues(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_POLICY", "verify")

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	storage := &fakeStorage{}
	ledger := &fakeLedger{} // empty → verify will fail with missing-all
	args := []string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "test goal",
		"--cycles", "1",
	}
	report := "Build status: FAIL — tests RED\n"
	rc, _, stderr := runM4Loop(t, projectRoot, evolveDir, args, storage, ledger, nil, report, "", 1)

	// rc=3 (M5+): classifier said build-fail (recoverable). policy=verify
	// continues the loop, records the failure to state.json (or WARNs if
	// missing), increments RecoverableFailures, and returns rc=3 at batch
	// end — bash dispatcher parity (DISPATCH_RC=3 = DONE-WITH-RECOVERABLE-FAILURES).
	if rc != 3 {
		t.Fatalf("rc=%d want 3 (recoverable continue → DONE-WITH-RECOVERABLE-FAILURES); stderr=%q", rc, stderr)
	}
	// abnormal-events.jsonl in the workspace must have verify-failed +
	// classification events.
	events := readAbnormalEvents(t, cycleWorkspace(projectRoot, 1))
	if got := countEvents(events, "verify-failed"); got != 1 {
		t.Fatalf("verify-failed count=%d want 1; events=%+v", got, events)
	}
	if got := countEvents(events, "classification"); got != 1 {
		t.Fatalf("classification count=%d want 1", got)
	}
}

// TestRunLoop_PolicyVerify_IntegrityBreachStops seeds a workspace with
// NO orchestrator-report (classifier → integrity-breach) and an empty
// ledger. policy=verify must STOP with rc=2.
func TestRunLoop_PolicyVerify_IntegrityBreachStops(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_POLICY", "verify")

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	storage := &fakeStorage{}
	ledger := &fakeLedger{}

	args := []string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "test goal",
		"--cycles", "1",
	}
	defer installStubDeps(t, storage, ledger)()
	// Create the workspace dir but DON'T write orchestrator-report.md
	// — that's what triggers integrity-breach.
	if err := os.MkdirAll(cycleWorkspace(projectRoot, 1), 0o755); err != nil {
		t.Fatalf("mkdir ws: %v", err)
	}
	var stdout, stderr bytes.Buffer
	rc := runLoop(args, nil, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("rc=%d want 2 (integrity-breach); stderr=%q", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), "integrity_breach") {
		t.Fatalf("stdout should mention integrity_breach: %q", stdout.String())
	}
}

// TestRunLoop_PolicyStop_AnyVerifyFailStops verifies that the legacy
// fail-fast policy STOPs on any verify failure regardless of class.
func TestRunLoop_PolicyStop_AnyVerifyFailStops(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_POLICY", "stop")

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	storage := &fakeStorage{}
	ledger := &fakeLedger{}
	args := []string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "test goal",
		"--cycles", "1",
	}
	// EPERM marker = infrastructure (would be recoverable under
	// policy=verify); policy=stop must still halt the batch.
	report := "EPERM: sandbox denied write\n"
	rc, stdout, stderr := runM4Loop(t, projectRoot, evolveDir, args, storage, ledger, nil, report, "", 1)
	if rc != 2 {
		t.Fatalf("rc=%d want 2 (policy=stop); stderr=%q", rc, stderr)
	}
	if !strings.Contains(stdout, "verify_failed_stop") {
		t.Fatalf("stop_reason should be verify_failed_stop: stdout=%q", stdout)
	}
}

// TestRunLoop_VerifyOK_NoEvents covers the success path — a fully
// populated ledger means verify passes, no abnormal events get
// emitted, rc=0.
func TestRunLoop_VerifyOK_NoEvents(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_POLICY", "verify")
	t.Setenv("EVOLVE_DISPATCH_REPEAT_THRESHOLD", "")

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	storage := &fakeStorage{state: core.State{LastCycleNumber: 0}}
	ledger := &fakeLedger{entries: []core.LedgerEntry{
		{Cycle: 1, Role: "scout", Kind: "agent_subprocess", ExitCode: 0},
		{Cycle: 1, Role: "builder", Kind: "agent_subprocess", ExitCode: 0},
		{Cycle: 1, Role: "auditor", Kind: "agent_subprocess", ExitCode: 0},
	}}
	args := []string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "test goal",
		"--cycles", "1",
	}

	rc, _, stderr := runM4Loop(t, projectRoot, evolveDir, args, storage, ledger, nil, "OK\n", "", 1)
	if rc != 0 {
		t.Fatalf("rc=%d want 0; stderr=%q", rc, stderr)
	}
	events := readAbnormalEvents(t, cycleWorkspace(projectRoot, 1))
	// counter-non-advance MAY fire because our fake orchestrator
	// doesn't actually bump state.LastCycleNumber the same way a real
	// run would when followed by a PASS audit. That's expected. But
	// verify-failed must NOT have fired.
	if got := countEvents(events, "verify-failed"); got != 0 {
		t.Fatalf("verify-failed count=%d want 0; events=%+v", got, events)
	}
}

// TestUpdateBreaker tabulates the step function in isolation. The
// integration in runLoop is exercised via TestRunLoop_CircuitBreakerTrips,
// which drives a stuck orchestrator through a fakeStorage that does
// not advance LastCycleNumber.
func TestUpdateBreaker(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                              string
		prev, streak, ranCycle, threshold int
		wantPrev, wantStreak              int
		wantTrip                          bool
	}{
		{"first call", -1, 0, 1, 5, 1, 1, false},
		{"same cycle increments streak", 1, 1, 1, 5, 1, 2, false},
		{"different cycle resets streak", 1, 4, 2, 5, 2, 1, false},
		{"streak reaches threshold trips", 3, 4, 3, 5, 3, 5, true},
		{"streak past threshold stays tripped", 3, 6, 3, 5, 3, 7, true},
		{"threshold of 1 trips immediately", -1, 0, 1, 1, 1, 1, true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, s, trip := updateBreaker(tc.prev, tc.streak, tc.ranCycle, tc.threshold)
			if p != tc.wantPrev || s != tc.wantStreak || trip != tc.wantTrip {
				t.Fatalf("got (prev=%d streak=%d trip=%v), want (prev=%d streak=%d trip=%v)",
					p, s, trip, tc.wantPrev, tc.wantStreak, tc.wantTrip)
			}
		})
	}
}

// stuckStorage is a fakeStorage variant whose WriteState is a no-op.
// The orchestrator's RunCycle calls WriteState(state) at the end to
// bump LastCycleNumber; stuckStorage ignores this so result.Cycle
// stays at 1 every iteration, simulating a state.json that the OS
// sandbox refuses to write (the bash scenario the breaker was added
// to catch).
type stuckStorage struct{ fakeStorage }

func (s *stuckStorage) WriteState(context.Context, core.State) error { return nil }

// TestRunLoop_CircuitBreakerTrips drives the integration: stuckStorage
// keeps result.Cycle=1 forever, threshold=3, cycles=5 → breaker trips
// on iteration 3, exits with rc=1 + stop_reason=circuit_breaker.
func TestRunLoop_CircuitBreakerTrips(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_POLICY", "off") // skip verify so only the breaker can fire
	t.Setenv("EVOLVE_DISPATCH_REPEAT_THRESHOLD", "3")

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	storage := &stuckStorage{}
	ledger := &fakeLedger{}
	defer installStubDeps(t, storage, ledger)()

	// Pre-create workspace cycle-1 so EmitCircuitBreakerTripped can
	// write its abnormal event.
	if err := os.MkdirAll(cycleWorkspace(projectRoot, 1), 0o755); err != nil {
		t.Fatalf("mkdir ws: %v", err)
	}

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "test goal",
		"--cycles", "5",
	}, nil, &stdout, &stderr)

	if rc != 1 {
		t.Fatalf("rc=%d want 1 (circuit_breaker); stderr=%q", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), "circuit_breaker") {
		t.Fatalf("stop_reason should be circuit_breaker; stdout=%q", stdout.String())
	}
	events := readAbnormalEvents(t, cycleWorkspace(projectRoot, 1))
	if got := countEvents(events, "circuit-breaker-tripped"); got != 1 {
		t.Fatalf("circuit-breaker-tripped count=%d want 1; events=%+v", got, events)
	}
}

// readAbnormalEvents parses every line of abnormal-events.jsonl in
// workspace. Returns nil on missing file.
func readAbnormalEvents(t *testing.T, workspace string) []map[string]any {
	t.Helper()
	f, err := os.Open(filepath.Join(workspace, "abnormal-events.jsonl"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	var out []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e map[string]any
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("unmarshal %q: %v", sc.Text(), err)
		}
		out = append(out, e)
	}
	return out
}

func countEvents(events []map[string]any, eventType string) int {
	n := 0
	for _, e := range events {
		if e["event_type"] == eventType {
			n++
		}
	}
	return n
}

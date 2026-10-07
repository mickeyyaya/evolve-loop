package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// fakeLedgerNoAppend wraps fixtures.FakeLedger but makes Append a deliberate
// no-op, so verify reads only the test-controlled Entries slice, not the
// kind:"phase" entries the stub orchestrator happens to write during a no-op
// run — an accumulating Append would make every no-op run look like a
// complete cycle and silently defeat the failure-path tests.
type fakeLedgerNoAppend struct {
	*fixtures.FakeLedger
	lifecycle []ledger.LifecycleRecord // the inbox walk's lines, recorded so wiring proofs can see them
}

func (fakeLedgerNoAppend) Append(context.Context, core.LedgerEntry) error { return nil }

// AppendLifecycle satisfies rootLedger (the inbox mover's chained seam).
func (f *fakeLedgerNoAppend) AppendLifecycle(_ context.Context, r ledger.LifecycleRecord) error {
	f.lifecycle = append(f.lifecycle, r)
	return nil
}

func newFakeLedger() *fakeLedgerNoAppend {
	return &fakeLedgerNoAppend{FakeLedger: &fixtures.FakeLedger{}}
}

// scriptedOrch is a *core.Orchestrator stand-in that returns canned cycle
// results in sequence. The real Orchestrator type is a struct, not an
// interface, so a test seam has to replace wireOrchestratorDepsFn entirely.
type scriptedOrch struct {
	results []core.CycleResult
	errs    []error
	storage *fixtures.FakeStorage
	ledger  *fakeLedgerNoAppend
	idx     int
}

// noopRunner satisfies core.PhaseRunner by returning PASS for every phase
// without touching disk or invoking a CLI, letting the orchestrator's state
// machine traverse start→end while the test controls the ledger contents and
// the per-cycle workspace.
type noopRunner struct{ name string }

func (n noopRunner) Name() string { return n.name }
func (n noopRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	if n.name == string(core.PhaseBuild) && req.ExplanationDocumentationVersion != 0 {
		if err := os.MkdirAll(req.Workspace, 0o755); err != nil {
			return core.PhaseResponse{}, err
		}
		report := explanationdocs.RenderNotApplicableDeclaration("the base-bound Build diff contains no material changes")
		if err := os.WriteFile(filepath.Join(req.Workspace, "build-report.md"), []byte(report), 0o644); err != nil {
			return core.PhaseResponse{}, err
		}
	}
	return core.PhaseResponse{Verdict: core.VerdictPASS}, nil
}

func initLoopContractRepo(t *testing.T, projectRoot string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(projectRoot, ".git")); err == nil {
		return
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".gitignore"), []byte(".evolve/\ngo/bin/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	initRepo(t, projectRoot)
}

// installStubDeps swaps wireOrchestratorDepsFn for one backed by a
// noop-PASS orchestrator: every phase is a no-op, so the only ledger entries
// are the phase-kind appends the orchestrator writes itself, and verify will
// fail unless the test pre-seeds agent_subprocess entries via
// fixtures.FakeLedger.Entries.
func installStubDeps(t *testing.T, storage core.Storage, ledger rootLedger) func() {
	t.Helper()
	prev := wireOrchestratorDepsFn
	wireOrchestratorDepsFn = func(projectRoot, evolveDir string, console io.Writer, run routingRun) orchDeps {
		initLoopContractRepo(t, projectRoot)
		runners := map[core.Phase]core.PhaseRunner{
			core.PhaseIntent:       noopRunner{name: "intent"},
			core.PhaseScout:        noopRunner{name: "scout"},
			core.PhaseTriage:       noopRunner{name: "triage"},
			core.PhaseTDD:          noopRunner{name: "tdd"},
			core.PhaseBuildPlanner: noopRunner{name: "build-planner"},
			core.PhaseBuild:        noopRunner{name: "build"},
			core.PhaseAudit:        noopRunner{name: "audit"},
			core.PhaseShip:         noopRunner{name: "ship"},
			core.PhaseRetro:        noopRunner{name: "retro"},
		}
		return orchDeps{
			// Signals uses the same constructor as production, so its rendered
			// lines prove production's.
			// See ADR-0101.
			Signals:      newRootSignalCenter(projectRoot, evolveDir, console),
			Storage:      storage,
			Ledger:       ledger,
			Orchestrator: core.NewOrchestrator(storage, ledger, runners),
		}
	}
	return func() { wireOrchestratorDepsFn = prev }
}

func TestResolveDispatchPolicy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  dispatchPolicy
	}{
		{"default → verify", "", dispatchPolicyVerify},
		{"explicit verify", "verify", dispatchPolicyVerify},
		{"explicit off", "off", dispatchPolicyOff},
		{"explicit stop", "stop", dispatchPolicyStop},
		{"unknown → verify (fallback)", "bogus", dispatchPolicyVerify},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stderr bytes.Buffer
			got := resolveDispatchPolicy(tc.input, &stderr)
			if got != tc.want {
				t.Fatalf("policy(%q)=%v want %v (stderr=%q)", tc.input, got, tc.want, stderr.String())
			}
		})
	}
}

func TestResolveCircuitBreakerThreshold(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input int
		want  int
	}{
		{0, defaultCircuitBreakerThreshold},
		{3, 3},
		{100, 100},
		{-5, defaultCircuitBreakerThreshold},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%d", tc.input), func(t *testing.T) {
			t.Parallel()
			if got := resolveCircuitBreakerThreshold(tc.input); got != tc.want {
				t.Fatalf("threshold(%d)=%d want %d", tc.input, got, tc.want)
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
	s := &fixtures.FakeStorage{State: core.State{LastCycleNumber: 42}}
	n, err := readLastCycleNumber(context.Background(), s)
	if err != nil || n != 42 {
		t.Fatalf("got n=%d err=%v", n, err)
	}
}

// helperPrepWorkspace seeds the minimum files the post-cycle pipeline reads:
// orchestrator-report.md (drives classification) and optionally
// .cycle-verdict (drives the memo gate).
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

// runM4Loop seeds the cycle workspace and lets the caller pre-seed the fake
// ledger/storage before running the loop.
func runM4Loop(t *testing.T, projectRoot, evolveDir string, args []string,
	storage *fixtures.FakeStorage, ledger *fakeLedgerNoAppend,
	beforeRun func(*fixtures.FakeStorage, *fakeLedgerNoAppend),
	report, verdict string,
	cycleNum int,
) (int, string, string) {
	t.Helper()
	defer installStubDeps(t, storage, ledger)()

	helperPrepWorkspace(t, projectRoot, cycleNum, report, verdict)
	if beforeRun != nil {
		beforeRun(storage, ledger)
	}

	var stdout, stderr bytes.Buffer
	rc := runLoop(args, nil, &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

func TestRunLoop_PolicyOff_SkipsVerify(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeDispatchPolicy(t, evolveDir, "off")
	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger()

	args := []string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "test goal",
		"--cycles", "1",
	}
	rc, _, stderr := runM4Loop(t, projectRoot, evolveDir, args, storage, ledger, nil, "", "", 1)

	if rc != 0 {
		t.Fatalf("rc=%d want 0; stderr=%q", rc, stderr)
	}
}

func TestRunLoop_PolicyVerify_RecoverableContinues(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeDispatchPolicy(t, evolveDir, "verify")
	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger() // empty → verify will fail with missing-all
	args := []string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "test goal",
		"--cycles", "1",
	}
	report := "Build status: FAIL — tests RED\n"
	rc, _, stderr := runM4Loop(t, projectRoot, evolveDir, args, storage, ledger, nil, report, "", 1)

	// rc=3: classified build-fail (recoverable); policy=verify continues
	// rather than halting.
	if rc != 3 {
		t.Fatalf("rc=%d want 3 (recoverable continue → DONE-WITH-RECOVERABLE-FAILURES); stderr=%q", rc, stderr)
	}
	events := readAbnormalEvents(t, cycleWorkspace(projectRoot, 1))
	if got := countEvents(events, "verify-failed"); got != 1 {
		t.Fatalf("verify-failed count=%d want 1; events=%+v", got, events)
	}
	if got := countEvents(events, "classification"); got != 1 {
		t.Fatalf("classification count=%d want 1", got)
	}
}

func TestRunLoop_PolicyVerify_AcceptsCompletedEmptyTriageChain(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeDispatchPolicy(t, evolveDir, "verify")
	storage := &fixtures.FakeStorage{}
	ledger := &fakeLedgerNoAppend{FakeLedger: &fixtures.FakeLedger{Entries: []core.LedgerEntry{
		{Cycle: 1, Role: "scout", Kind: "phase", ExitCode: 0},
		{Cycle: 1, Role: "triage", Kind: "phase", ExitCode: 0},
	}}}
	args := []string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "protected task",
		"--cycles", "1",
	}
	beforeRun := func(_ *fixtures.FakeStorage, _ *fakeLedgerNoAppend) {
		handoff := filepath.Join(cycleWorkspace(projectRoot, 1), "handoff-triage.json")
		if err := os.WriteFile(handoff, []byte(`{"cycle_size":"small","deliverable_kind":"code","phase_skip":[]}`), 0o644); err != nil {
			t.Fatalf("write triage handoff: %v", err)
		}
		path := filepath.Join(cycleWorkspace(projectRoot, 1), "triage-decision.json")
		if err := os.WriteFile(path, []byte(`{"top_n":[],"deferred":[{"id":"protected","reason":"source surface is protected"}]}`), 0o644); err != nil {
			t.Fatalf("write triage decision: %v", err)
		}
	}

	rc, _, stderr := runM4Loop(t, projectRoot, evolveDir, args, storage, ledger, beforeRun, "Triage completed with no authorized work.\n", "", 1)
	if rc != 0 {
		t.Fatalf("rc=%d want 0 for an explicit empty Triage commitment; stderr=%q", rc, stderr)
	}
	if strings.Contains(stderr, "classification=infrastructure") {
		t.Fatalf("planned no-work cycle was classified as infrastructure: %q", stderr)
	}
}

func TestRunLoop_PolicyVerify_RejectsNullTriageCommitment(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeDispatchPolicy(t, evolveDir, "verify")
	storage := &fixtures.FakeStorage{}
	ledger := &fakeLedgerNoAppend{FakeLedger: &fixtures.FakeLedger{Entries: []core.LedgerEntry{
		{Cycle: 1, Role: "scout", Kind: "phase", ExitCode: 0},
		{Cycle: 1, Role: "triage", Kind: "phase", ExitCode: 0},
	}}}
	args := []string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "protected task",
		"--cycles", "1",
	}
	beforeRun := func(_ *fixtures.FakeStorage, _ *fakeLedgerNoAppend) {
		ws := cycleWorkspace(projectRoot, 1)
		if err := os.WriteFile(filepath.Join(ws, "handoff-triage.json"), []byte(`{"cycle_size":"small"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(`{"top_n":null}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	rc, _, _ := runM4Loop(t, projectRoot, evolveDir, args, storage, ledger, beforeRun, "Triage completed with no authorized work.\n", "", 1)
	if rc == 0 {
		t.Fatal("null top_n was accepted as a completed empty-Triage chain")
	}
}

func TestCompletedTriageNoWorkRequiresHostProvenance(t *testing.T) {
	t.Parallel()
	valid := core.CycleResult{
		FinalVerdict:      core.CycleOutcomeSkippedUnknown,
		TerminationReason: core.CycleTerminationTriageNoWork,
		PhasesRun:         []core.Phase{core.PhaseScout, core.PhaseTriage},
	}
	cases := []struct {
		name   string
		result core.CycleResult
		want   bool
	}{
		{name: "host-authorized-terminal-triage", result: valid, want: true},
		{name: "artifact-alone-has-no-host-reason", result: core.CycleResult{FinalVerdict: core.CycleOutcomeSkippedUnknown, PhasesRun: valid.PhasesRun}},
		{name: "failed-triage-is-not-no-work", result: core.CycleResult{FinalVerdict: core.VerdictFAIL, TerminationReason: core.CycleTerminationTriageNoWork, PhasesRun: valid.PhasesRun}},
		{name: "downstream-phase-ran", result: core.CycleResult{FinalVerdict: core.CycleOutcomeSkippedUnknown, TerminationReason: core.CycleTerminationTriageNoWork, PhasesRun: []core.Phase{core.PhaseScout, core.PhaseTriage, core.PhaseTDD}}},
		{name: "implementation-ran-before-triage", result: core.CycleResult{FinalVerdict: core.CycleOutcomeSkippedUnknown, TerminationReason: core.CycleTerminationTriageNoWork, PhasesRun: []core.Phase{core.PhaseScout, core.PhaseTDD, core.PhaseTriage}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := completedTriageNoWork(sequentialCycle{result: tc.result}); got != tc.want {
				t.Errorf("completedTriageNoWork() = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestRunLoop_PolicyVerify_IntegrityBreachStops(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeDispatchPolicy(t, evolveDir, "verify")
	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger()

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

func TestRunLoop_PolicyStop_AnyVerifyFailStops(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeDispatchPolicy(t, evolveDir, "stop")
	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger()
	args := []string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "test goal",
		"--cycles", "1",
	}
	// EPERM classifies as infrastructure, recoverable under policy=verify, but
	// policy=stop must still halt the batch.
	report := "EPERM: sandbox denied write\n"
	rc, stdout, stderr := runM4Loop(t, projectRoot, evolveDir, args, storage, ledger, nil, report, "", 1)
	if rc != 2 {
		t.Fatalf("rc=%d want 2 (policy=stop); stderr=%q", rc, stderr)
	}
	if !strings.Contains(stdout, "verify_failed_stop") {
		t.Fatalf("stop_reason should be verify_failed_stop: stdout=%q", stdout)
	}
}

func TestRunLoop_VerifyOK_NoEvents(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeDispatchPolicy(t, evolveDir, "verify")
	storage := &fixtures.FakeStorage{State: core.State{LastCycleNumber: 0}}
	ledger := &fakeLedgerNoAppend{FakeLedger: &fixtures.FakeLedger{Entries: []core.LedgerEntry{
		{Cycle: 1, Role: "scout", Kind: "agent_subprocess", ExitCode: 0},
		{Cycle: 1, Role: "builder", Kind: "agent_subprocess", ExitCode: 0},
		{Cycle: 1, Role: "auditor", Kind: "agent_subprocess", ExitCode: 0},
	}}}
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
	// counter-non-advance may fire, since the fake orchestrator doesn't bump
	// state.LastCycleNumber the way a real PASS-audit run would; only
	// verify-failed is asserted here.
	if got := countEvents(events, "verify-failed"); got != 0 {
		t.Fatalf("verify-failed count=%d want 0; events=%+v", got, events)
	}
}

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

// stuckStorage ignores WriteState, so result.Cycle stays at 1 every
// iteration, simulating a state.json the OS sandbox refuses to write — the
// scenario the circuit breaker exists to catch.
type stuckStorage struct{ fixtures.FakeStorage }

func (s *stuckStorage) WriteState(context.Context, core.State) error { return nil }

func TestRunLoop_CircuitBreakerTrips(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeDispatchPolicyFull(t, evolveDir, "off", 3) // skip verify so only the breaker can fire
	storage := &stuckStorage{}
	ledger := newFakeLedger()
	defer installStubDeps(t, storage, ledger)()

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

package core

// RED tests for cycle-171 T1 (phase-timing-json + phase_retry ledger) and T2
// (structured-failure-diag). White-box (package core) so they reuse the existing
// fakeStorage / fakeLedger / buildRunners / wrapTimeout harness in
// orchestrator_test.go. They reference ONLY already-public symbols so the core
// test binary still COMPILES at the pre-implementation baseline — they fail at
// RUNTIME (file missing / no ledger entry), which is the correct RED signal and
// does not break sibling tests. Builder makes them GREEN by writing the
// phase-timing.json accumulator, the phase_retry ledger append, and the
// <phase>-failure-diag.json writer in orchestrator.go.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/policy"
)

// cycleWorkspaceDir mirrors RunCycle's WorkspacePath formula
// (orchestrator.go: "%s/.evolve/runs/cycle-%d").
func cycleWorkspaceDir(root string, cycle int) string {
	return filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
}

// T1 / AC-1+AC-2: after a full cycle runs, <workspace>/phase-timing.json must
// exist, be a JSON array with one entry per phase that ran, and each entry must
// carry the load-bearing fields (phase, duration_ms, verdict). The phase-name
// subset assertion is the anti-no-op guard: an empty or stub file cannot satisfy
// it because it pins the entries to the phases RunCycle actually executed.
func TestPhaseTimingJSON_WrittenAfterRunCycle(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	path := filepath.Join(cycleWorkspaceDir(root, res.Cycle), "phase-timing.json")
	data, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatalf("phase-timing.json must be written after RunCycle: %v", rerr)
	}

	var entries []map[string]any
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("phase-timing.json must be a JSON array: %v\n%s", err, data)
	}
	if len(entries) != len(res.PhasesRun) {
		t.Errorf("timing entries=%d, want one per phase run (%d): %v",
			len(entries), len(res.PhasesRun), res.PhasesRun)
	}

	seen := map[string]bool{}
	for _, e := range entries {
		if _, ok := e["phase"]; !ok {
			t.Errorf("timing entry missing 'phase' key: %v", e)
		}
		if _, ok := e["duration_ms"]; !ok {
			t.Errorf("timing entry missing 'duration_ms' key: %v", e)
		}
		if _, ok := e["verdict"]; !ok {
			t.Errorf("timing entry missing 'verdict' key: %v", e)
		}
		if p, ok := e["phase"].(string); ok {
			seen[p] = true
		}
	}
	for _, want := range []string{"scout", "build", "audit", "ship"} {
		if !seen[want] {
			t.Errorf("phase-timing.json missing entry for %q; got phases %v", want, seen)
		}
	}
}

// T1 / AC-3: a self-heal relaunch (ErrArtifactTimeout, recovers on attempt 2)
// must append a kind=phase_retry ledger entry naming the retried phase and
// carrying exit_code 81. Today only an os.Stderr WARN line is emitted (no
// structured audit trail) — so this is RED until Builder adds the append.
func TestPhaseTimingJSON_RetryEmitsLedgerEntry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	// One artifact-timeout then success → exactly one retry of scout.
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTimeout(), failUntil: 1}
	o := NewOrchestrator(st, led, runners)

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root}); err != nil {
		t.Fatalf("RunCycle should self-heal the transient timeout, got: %v", err)
	}

	var retry *LedgerEntry
	for i := range led.entries {
		if led.entries[i].Kind == "phase_retry" {
			retry = &led.entries[i]
			break
		}
	}
	if retry == nil {
		t.Fatalf("expected a kind=phase_retry ledger entry on self-heal retry; entries=%+v", led.entries)
	}
	if retry.Role != "scout" {
		t.Errorf("phase_retry role=%q, want scout", retry.Role)
	}
	if retry.ExitCode != 81 {
		t.Errorf("phase_retry exit_code=%d, want 81 (ErrArtifactTimeout sentinel)", retry.ExitCode)
	}
}

// T2 / AC-1+AC-2+AC-5: when a mandatory phase exhausts its retries and aborts,
// the orchestrator must write <workspace>/<phase>-failure-diag.json BEFORE
// returning the error, with phase, exit_code (81 for ErrArtifactTimeout), and a
// non-empty error_message. RED today: the abort path returns the wrapped error
// with no structured file.
func TestFailureDiag_WrittenOnPhaseAbort(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	// Times out on every attempt → aborts after phaseMaxAttempts.
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTimeout(), failUntil: 99}
	o := NewOrchestrator(st, led, runners)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root})
	if err == nil {
		t.Fatalf("RunCycle should abort after exhausting retries")
	}

	path := filepath.Join(cycleWorkspaceDir(root, res.Cycle), "scout-failure-diag.json")
	data, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatalf("scout-failure-diag.json must be written on phase abort: %v", rerr)
	}

	var diag map[string]any
	if err := json.Unmarshal(data, &diag); err != nil {
		t.Fatalf("failure-diag must be valid JSON: %v\n%s", err, data)
	}
	if diag["phase"] != "scout" {
		t.Errorf("diag phase=%v, want scout", diag["phase"])
	}
	if ec, ok := diag["exit_code"].(float64); !ok || int(ec) != 81 {
		t.Errorf("diag exit_code=%v, want 81 (ErrArtifactTimeout)", diag["exit_code"])
	}
	if msg, _ := diag["error_message"].(string); msg == "" {
		t.Errorf("diag error_message must be non-empty (was %q)", diag["error_message"])
	}
}

// T2 negative axis: a fully-PASS cycle must NOT emit any *-failure-diag.json.
// This guards against a no-op implementation that always writes the diag. It is
// GREEN at the pre-implementation baseline (no diag code yet) AND must stay
// GREEN after Builder wires the abort-only writer — it pins the "abort-only"
// contract.
func TestFailureDiag_NotWrittenOnPassingCycle(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(cycleWorkspaceDir(root, res.Cycle), "*-failure-diag.json"))
	if len(matches) != 0 {
		t.Errorf("no failure-diag may be written on a fully-PASS cycle; found %v", matches)
	}
}

type backfillRunner struct {
	content string
}

func (b *backfillRunner) Name() string { return "scout" }
func (b *backfillRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	cleanFile := filepath.Join(req.Workspace, "scout-stdout.clean.txt")
	_ = os.MkdirAll(req.Workspace, 0o755)
	_ = os.WriteFile(cleanFile, []byte(b.content), 0o644)
	return PhaseResponse{}, wrapTimeout()
}

func TestOrchestrator_Backfill_LedgerEntry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseScout] = &backfillRunner{
		content: "# Scout Report\n" + strings.Repeat("a", 250),
	}
	o := NewOrchestrator(st, led, runners)

	req := CycleRequest{
		ProjectRoot: root,
		Env:         map[string]string{},
	}
	_, err := o.RunCycle(context.Background(), req)
	if err != nil {
		t.Fatalf("RunCycle failed: %v", err)
	}

	var backfillEntry *LedgerEntry
	for i := range led.entries {
		if led.entries[i].Kind == "backfill" {
			backfillEntry = &led.entries[i]
			break
		}
	}
	if backfillEntry == nil {
		t.Fatalf("expected a kind=backfill ledger entry; entries=%+v", led.entries)
	}
}

func TestOrchestrator_Backfill_LedgerRole(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseScout] = &backfillRunner{
		content: "# Scout Report\n" + strings.Repeat("a", 250),
	}
	o := NewOrchestrator(st, led, runners, WithWorkflowConfig(policy.WorkflowConfig{BackfillEnabled: true}))

	req := CycleRequest{
		ProjectRoot: root,
		Env:         map[string]string{},
	}
	_, err := o.RunCycle(context.Background(), req)
	if err != nil {
		t.Fatalf("RunCycle failed: %v", err)
	}

	var backfillEntry *LedgerEntry
	for i := range led.entries {
		if led.entries[i].Kind == "backfill" {
			backfillEntry = &led.entries[i]
			break
		}
	}
	if backfillEntry == nil {
		t.Fatalf("expected a kind=backfill ledger entry")
	}
	if backfillEntry.Role != "scout" {
		t.Errorf("expected Role='scout', got %q", backfillEntry.Role)
	}
}

func TestOrchestrator_Backfill_NoLedgerEntryWhenDisabled(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseScout] = &backfillRunner{
		content: "# Scout Report\n" + strings.Repeat("a", 250),
	}
	o := NewOrchestrator(st, led, runners, WithWorkflowConfig(policy.WorkflowConfig{BackfillEnabled: false}))

	req := CycleRequest{
		ProjectRoot: root,
		Env:         map[string]string{},
	}
	_, err := o.RunCycle(context.Background(), req)
	if err == nil {
		t.Fatal("expected RunCycle to abort because backfill is disabled")
	}

	for i := range led.entries {
		if led.entries[i].Kind == "backfill" {
			t.Errorf("unexpected kind=backfill ledger entry found: %+v", led.entries[i])
		}
	}
}

type backfillRunnerGeneric struct {
	phase   string
	content string
}

func (b *backfillRunnerGeneric) Name() string { return b.phase }
func (b *backfillRunnerGeneric) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	cleanFile := filepath.Join(req.Workspace, b.phase+"-stdout.clean.txt")
	_ = os.MkdirAll(req.Workspace, 0o755)
	_ = os.WriteFile(cleanFile, []byte(b.content), 0o644)
	return PhaseResponse{}, wrapTimeout()
}

func TestOrchestrator_Backfill_TDDArtifactPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseTDD] = &backfillRunnerGeneric{
		phase:   "tdd",
		content: "# TDD\n" + strings.Repeat("a", 250),
	}
	o := NewOrchestrator(st, led, runners, WithWorkflowConfig(policy.WorkflowConfig{BackfillEnabled: true}))

	req := CycleRequest{
		ProjectRoot: root,
		Env:         map[string]string{},
	}
	res, err := o.RunCycle(context.Background(), req)
	if err != nil {
		t.Fatalf("RunCycle failed: %v", err)
	}

	workspace := cycleWorkspaceDir(root, res.Cycle)
	// Must write to test-report.md, not tdd-report.md
	testReportPath := filepath.Join(workspace, "test-report.md")
	if _, err := os.Stat(testReportPath); err != nil {
		t.Errorf("expected test-report.md to be written for tdd backfill: %v", err)
	}
	tddReportPath := filepath.Join(workspace, "tdd-report.md")
	if _, err := os.Stat(tddReportPath); err == nil {
		t.Errorf("unexpected tdd-report.md written for tdd backfill")
	}
}

func TestPhaseTimingJSON_AttemptCount(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	// Scout fails once then succeeds → exactly 2 attempts.
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTimeout(), failUntil: 1}
	// Build passes on first attempt → exactly 1 attempt.
	runners[PhaseBuild] = &fakeRunner{name: "build"}
	o := NewOrchestrator(st, led, runners)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	path := filepath.Join(cycleWorkspaceDir(root, res.Cycle), "phase-timing.json")
	data, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatalf("phase-timing.json must be written: %v", rerr)
	}

	var entries []map[string]any
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	seen := map[string]int{}
	for _, e := range entries {
		p, _ := e["phase"].(string)
		if ac, ok := e["attempt_count"].(float64); ok {
			seen[p] = int(ac)
		}
	}

	if seen["scout"] != 2 {
		t.Errorf("scout attempt_count=%d, want 2", seen["scout"])
	}
	if seen["build"] != 1 {
		t.Errorf("build attempt_count=%d, want 1", seen["build"])
	}
}

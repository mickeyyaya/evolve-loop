package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// Orchestrator phase-1 test surface — uses fake adapters to verify the
// sequencing contract without touching disk or processes. Real adapter
// impls land in Phase 2.

// --- fakes ---
//
// These stay local (not migrated to go/test/fixtures, which has the canonical
// FakeStorage/FakeLedger/FakeRunner): this is a white-box `package core` test
// that also exercises unexported internals (recordAuditBinding, runGit, …), and
// fixtures imports core — importing it here would be a cycle. This is the one
// place the test-double dedup deliberately cannot reach.

type fakeStorage struct {
	state                      State
	cycleState                 CycleState
	cycleStateLog              []CycleState
	stateLog                   []State
	lockHeld                   bool
	lockCount                  int
	mu                         sync.Mutex
	lockErr                    error
	failOnWriteCS              bool
	failOnReadState            bool
	failOnWriteState           bool
	writeCSFailAt              int // 0 = never; N = N-th write
	writeCSCalls               int
	enforceExplanationContract bool
}

func (f *fakeStorage) disableFreshExplanationContractForTest() bool {
	return !f.enforceExplanationContract
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
	// failErr (when set) is returned for the first failUntil calls, then the
	// runner succeeds. Models a transient phase failure for the self-heal
	// retry path (Fix D). failErr nil → never fails (default).
	failErr   error
	failUntil int
}

type requiredExplanationBuilder struct{}

func (*requiredExplanationBuilder) Name() string { return string(PhaseBuild) }
func (*requiredExplanationBuilder) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	if err := os.WriteFile(filepath.Join(req.Worktree, "feature.txt"), []byte("implemented\n"), 0o644); err != nil {
		return PhaseResponse{}, err
	}
	document, err := explanationdocs.DocumentPath(req.Cycle, req.RunID)
	if err != nil {
		return PhaseResponse{}, err
	}
	var changedAreas strings.Builder
	for _, path := range changedWorktreePathsSince(ctx, req.Worktree, req.WorktreeBaseSHA) {
		fmt.Fprintf(&changedAreas, "- `%s` — records a material artifact exercised by the fresh-cycle fixture.\n", path)
	}
	body := fmt.Sprintf(`# Build Explanation

## Build Binding
- Cycle: %d
- Base SHA: %s

## Summary
Adds the feature fixture used to exercise the fresh-cycle contract.

## Rationale
The material Build change requires a cycle-owned explanation.

## Changed Areas
%s

## Design Decisions
The fixture uses one plain file because no abstraction is needed.

## Verification
The orchestrator test exercises the full Build handoff floor.

## Compatibility
The fixture does not change a public interface.

## Limitations
The files exist only inside this isolated test repository.
`, req.Cycle, req.WorktreeBaseSHA, changedAreas.String())
	path := filepath.Join(req.Worktree, filepath.FromSlash(document))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return PhaseResponse{}, err
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return PhaseResponse{}, err
	}
	report := "## Explanation Documentation\n- Status: REQUIRED\n- Document: " + document + "\n"
	if err := os.WriteFile(filepath.Join(req.Workspace, "build-report.md"), []byte(report), 0o644); err != nil {
		return PhaseResponse{}, err
	}
	return PhaseResponse{Phase: string(PhaseBuild), Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

type normalizingExplanationBuilder struct{}

func (*normalizingExplanationBuilder) Name() string { return string(PhaseBuild) }
func (*normalizingExplanationBuilder) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	// Intentionally leave valid but non-gofmt source behind. The host owns this
	// normalization, so the explanation must be sealed only after it runs.
	if err := os.WriteFile(filepath.Join(req.Worktree, "go", "feature.go"), []byte("package fixture\n\nfunc Answer()int{return 42}\n"), 0o644); err != nil {
		return PhaseResponse{}, err
	}
	return (&requiredExplanationBuilder{}).Run(ctx, req)
}

type verifyingExplanationAudit struct {
	calls int
}

func (*verifyingExplanationAudit) Name() string { return string(PhaseAudit) }
func (r *verifyingExplanationAudit) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	r.calls++
	binding := explanationdocs.CycleBinding{
		ProjectRoot: req.ProjectRoot, Worktree: req.Worktree, Workspace: req.Workspace,
		BaseSHA: req.WorktreeBaseSHA, Cycle: req.Cycle, RunID: req.RunID,
		ContractVersion: req.ExplanationDocumentationVersion,
	}
	if _, active, err := explanationdocs.Verify(ctx, binding); err != nil || !active {
		return PhaseResponse{}, fmt.Errorf("first Audit received stale Build explanation: active=%v err=%v", active, err)
	}
	formatted, err := os.ReadFile(filepath.Join(req.Worktree, "go", "feature.go"))
	if err != nil {
		return PhaseResponse{}, err
	}
	if string(formatted) != "package fixture\n\nfunc Answer() int { return 42 }\n" {
		return PhaseResponse{}, fmt.Errorf("first Audit ran before gofmt normalization: %q", formatted)
	}
	return PhaseResponse{Phase: string(PhaseAudit), Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func (f *fakeRunner) Name() string { return f.name }
func (f *fakeRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	f.calls++
	f.requests = append(f.requests, req)
	if f.failErr != nil && f.calls <= f.failUntil {
		return PhaseResponse{}, f.failErr
	}
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
		ProjectRoot: t.TempDir(),
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
		ProjectRoot: t.TempDir(),
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
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), Env: envIn})
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

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
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

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
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

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
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

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err == nil {
		t.Fatal("expected error for missing scout runner")
	}
}

func TestOrchestrator_AdvancesLastCycleNumber(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 41}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
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
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err == nil {
		t.Fatal("ReadState error must propagate")
	}
}

func TestOrchestrator_InitialWriteCycleStateError(t *testing.T) {
	st := &fakeStorage{failOnWriteCS: true}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
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
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err == nil {
		t.Fatal("mid-phase WriteCycleState error must propagate")
	}
}

func TestOrchestrator_WriteCycleStatePostPhaseError(t *testing.T) {
	// Init=1, pre-scout=2, post-scout=3 → fail at 3.
	st := &fakeStorage{writeCSFailAt: 3}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err == nil {
		t.Fatal("post-phase WriteCycleState error must propagate")
	}
}

func TestOrchestrator_LedgerAppendError(t *testing.T) {
	st := &fakeStorage{}
	led := &fakeLedger{failOnAppend: true}
	o := NewOrchestrator(st, led, buildRunners(nil))
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err == nil {
		t.Fatal("ledger append error must propagate")
	}
}

func TestOrchestrator_FinalWriteStateError(t *testing.T) {
	st := &fakeStorage{failOnWriteState: true}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
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
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
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
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err == nil {
		t.Fatal("non-canonical verdict must be rejected")
	}
}

func TestOrchestrator_RecordsCompletedPhases(t *testing.T) {
	st := &fakeStorage{}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
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

func TestOrchestrator_FreshCycleActivatesBuildExplanationContract(t *testing.T) {
	st := &fakeStorage{enforceExplanationContract: true}
	runners := buildRunners(nil)
	runners[PhaseBuild] = &requiredExplanationBuilder{}
	o := NewOrchestrator(st, &fakeLedger{}, runners)
	root := initTempGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".evolve/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".gitignore")
	runGit(t, root, "commit", "-q", "-m", "base")

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if st.cycleState.ExplanationDocumentationVersion != explanationdocs.CurrentContractVersion {
		t.Fatalf("cycle contract version=%d, want %d", st.cycleState.ExplanationDocumentationVersion, explanationdocs.CurrentContractVersion)
	}
	marker := filepath.Join(root, ".evolve", "build-explanation-contracts", fmt.Sprintf("cycle-%d.json", st.cycleState.CycleID))
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("fresh cycle activation marker: %v", err)
	}
}

func TestOrchestrator_NormalizesBuildBeforeSealingExplanationForFirstAudit(t *testing.T) {
	st := &fakeStorage{enforceExplanationContract: true}
	runners := buildRunners(nil)
	runners[PhaseBuild] = &normalizingExplanationBuilder{}
	audit := &verifyingExplanationAudit{}
	runners[PhaseAudit] = audit
	o := NewOrchestrator(st, &fakeLedger{}, runners)
	root := initTempGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".evolve/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go", "go.mod"), []byte("module example.com/fixture\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".gitignore", "go/go.mod")
	runGit(t, root, "commit", "-q", "-m", "base")

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if audit.calls != 1 {
		t.Fatalf("Audit calls=%d, want first-attempt success", audit.calls)
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

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
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

// PhaseEnables["intent"]="on" in WorkflowConfig triggers the intent phase
// before Scout. CycleState.IntentRequired is persisted so resume +
// downstream consumers can read it.
func TestOrchestrator_IntentGate_PhaseEnableRunsIntentFirst(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners, WithWorkflowConfig(policy.WorkflowConfig{
		PhaseEnables: map[string]string{"intent": "on"},
	}))

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(),
		Env:         map[string]string{},
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
		ProjectRoot: t.TempDir(),
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
// Renamed from TestOrchestrator_RetroPASS_RoutesToShip, whose fixture comment read
// "Even with prior failures, retro PASS overrides and ships" — the clearest
// statement of the category error being corrected. A retro verdict reports whether
// the POST-MORTEM is complete; it cannot override a failure it merely described,
// and the retro persona is read-only outside its own artifacts, so the tree here is
// byte-identical to the one the auditor rejected.
func TestOrchestrator_RetroPASS_DoesNotRouteToShip(t *testing.T) {
	st := &fakeStorage{state: State{
		LastCycleNumber: 0,
		FailedAt: []FailedRecord{
			// Prior failures stand: a well-written retrospective does not clear them.
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

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	// After retro PASS, ship should have run.
	wantTail := []Phase{PhaseAudit, PhaseRetro}
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
	// Disposition contract (cycle-1046 verdict-path wiring): fixtures carry no
	// disposition.json, so the gate prefixes its loud reason — the branch
	// decision must still be carried (suffix), and the gate must be audible.
	for _, p := range res.PhasesRun {
		if p == PhaseShip {
			t.Fatalf("ship ran after an audit FAIL that only a retrospective 'recovered'; phases=%v", res.PhasesRun)
		}
	}
	if strings.Contains(res.RetroDecision, "retro-recovered") {
		t.Errorf("RetroDecision=%q still claims recovery", res.RetroDecision)
	}
	if !strings.Contains(res.RetroDecision, "disposition-gate:") {
		t.Errorf("RetroDecision=%q must surface the disposition-gate reason when no disposition was delivered", res.RetroDecision)
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

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	// Retro is the last phase — no ship, no second tdd.
	last := res.PhasesRun[len(res.PhasesRun)-1]
	if last != PhaseRetro {
		t.Errorf("last phase=%s, want retro", last)
	}
	if !strings.Contains(res.RetroDecision, "proceed:") {
		t.Errorf("RetroDecision=%q, want proceed: marker", res.RetroDecision)
	}
	retroReqs := runners[PhaseRetro].(*fakeRunner).requests
	if len(retroReqs) != 1 {
		t.Fatalf("retro requests = %d, want 1", len(retroReqs))
	}
	if got := retroReqs[0].Context["previous_verdict"]; got != VerdictFAIL {
		t.Errorf("retro previous_verdict context = %q, want FAIL", got)
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

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	// Fluent mode default → PROCEED with awareness → end.
	last := res.PhasesRun[len(res.PhasesRun)-1]
	if last != PhaseRetro {
		t.Errorf("last phase=%s, want retro (fluent proceed→end)", last)
	}
	if !strings.Contains(res.RetroDecision, "proceed:") {
		t.Errorf("RetroDecision=%q, want proceed: marker", res.RetroDecision)
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

// --- Fix D: self-heal on bridge ArtifactTimeout (exit=81) ---

// A phase that hits a bridge ArtifactTimeout once then succeeds must be
// relaunched, and the cycle must complete normally (cycle-149 exit=81 at scout
// aborted the whole loop with no retry).
func TestOrchestrator_PhaseArtifactTimeout_RetriesAndRecovers(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	// wrapTimeout() wraps ErrArtifactTimeout so errors.Is matches (mirrors engine.go).
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTimeout(), failUntil: 1}
	o := NewOrchestrator(st, led, runners)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle should self-heal the transient timeout, got: %v", err)
	}
	if got := runners[PhaseScout].(*fakeRunner).calls; got != 2 {
		t.Errorf("scout calls=%d, want 2 (one timeout + one successful relaunch)", got)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Errorf("verdict=%s, want PASS after recovery", res.FinalVerdict)
	}
	// The retry must not double-run downstream phases: each runs exactly once.
	for _, p := range []Phase{PhaseTriage, PhaseTDD, PhaseBuild, PhaseAudit, PhaseShip} {
		if got := runners[p].(*fakeRunner).calls; got != 1 {
			t.Errorf("phase %s calls=%d, want 1 (scout retry must not re-run downstream)", p, got)
		}
	}
}

// A phase that times out on every attempt must abort after the configured cap —
// not retry forever — and the error must still wrap ErrArtifactTimeout.
func TestOrchestrator_PhaseArtifactTimeout_AbortsAfterCap(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTimeout(), failUntil: 99}
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err == nil {
		t.Fatalf("RunCycle should abort after exhausting retries")
	}
	if !errors.Is(err, ErrArtifactTimeout) {
		t.Errorf("err=%v, want wrapped ErrArtifactTimeout", err)
	}
	if got := runners[PhaseScout].(*fakeRunner).calls; got != o.retryConfig.PhaseMaxAttempts {
		t.Errorf("scout calls=%d, want %d (capped)", got, o.retryConfig.PhaseMaxAttempts)
	}
}

// A non-timeout error must abort immediately with NO retry — retry is reserved
// for the recoverable artifact-timeout case.
func TestOrchestrator_PhaseNonTimeoutError_NoRetry(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: errors.New("deterministic boom"), failUntil: 99}
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err == nil {
		t.Fatalf("RunCycle should abort on a non-timeout error")
	}
	if got := runners[PhaseScout].(*fakeRunner).calls; got != 1 {
		t.Errorf("scout calls=%d, want 1 (no retry for non-timeout errors)", got)
	}
}

// wrapTimeout returns an error that wraps ErrArtifactTimeout the way the bridge
// engine does (fmt.Errorf("...: %w", ErrArtifactTimeout)).
func wrapTimeout() error {
	return errArtifactTimeoutWrapper{}
}

type errArtifactTimeoutWrapper struct{}

func (errArtifactTimeoutWrapper) Error() string {
	return "bridge: launch exit=81: " + ErrArtifactTimeout.Error()
}
func (errArtifactTimeoutWrapper) Unwrap() error { return ErrArtifactTimeout }

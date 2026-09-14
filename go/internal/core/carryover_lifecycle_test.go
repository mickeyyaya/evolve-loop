package core

// carryover_lifecycle_test.go — unit 03 (ADR-0103): the orchestrator's seam
// onto the carryover unit — the accessor pair, the persist facade reading the
// store at call time, the RMW branch's golden bytes, the consumer pin of the
// priority vocabulary, the one-construction guard and the closeout order at
// the real finalizeCycle.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// carryoverTodoExists keeps the nine assertion sites of this package's tests
// compiling; the rule lives in the unit as HasID.
func carryoverTodoExists(todos []CarryoverTodo, id string) bool { return carryover.HasID(todos, id) }

func TestCarryover_LiteralOrchestratorGetsTheLifecycleOnce(t *testing.T) {
	st := &fakeStorage{}
	o := &Orchestrator{storage: st}
	first := o.carryover()
	if first == nil || first.SignalsWired() {
		t.Fatalf("a literal orchestrator gets an unwired lifecycle: %v", first)
	}
	if o.carryover() != first {
		t.Fatal("the lazily built lifecycle is kept, not rebuilt")
	}
	o.writeFailureLearningState(context.Background(), &State{LastCycleNumber: 9})
	if st.state.LastCycleNumber != 9 {
		t.Fatalf("the persist facade lands in the legacy store: %+v", st.state)
	}
	wired := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(signalcenter.New()))
	if wired.carry == nil || !wired.carryover().SignalsWired() {
		t.Fatal("NewOrchestrator builds the lifecycle eagerly, wired to the root's Center")
	}
}

func TestCarryover_SeesASignalCenterAppliedAfterConstruction(t *testing.T) {
	o := NewOrchestrator(&fakeStorage{failOnWriteState: true}, &fakeLedger{}, buildRunners(nil))
	if o.carryover().SignalsWired() {
		t.Fatal("no Center at construction: unwired")
	}
	signals, got := recordingCenter()
	WithSignalCenter(signals)(o)
	if !o.carryover().SignalsWired() {
		t.Fatal("the late Center is seen through the accessor")
	}
	o.writeFailureLearningState(context.Background(), &State{LastCycleNumber: 3})
	warned := eventsOfKind(*got, signalcenter.KindCarryoverWarning)
	if len(warned) != 1 || warned[0].Code != carryover.CodePersistFailed || warned[0].Origin != "Lifecycle.Persist" || warned[0].Fields["step"] != "write" {
		t.Fatalf("a failed legacy write reaches the late Center: %+v", *got)
	}
}

// The default-build twin of the integration-tagged forensics test: the store
// is read at CALL time, never snapshotted at construction.
func TestWriteFailureLearningState_ReadsTheStoreAtCallTime(t *testing.T) {
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	swapped := &fakeStorage{}
	o.storage = swapped
	o.writeFailureLearningState(context.Background(), &State{LastCycleNumber: 7})
	if swapped.state.LastCycleNumber != 7 {
		t.Fatalf("the swapped store received the state: %+v", swapped.state)
	}
}

// ADR-0103 item 4: the RMW branch's bytes, captured on the pre-extraction code
// (f7c2d85d) with this exact fixture — the first core test to drive it.
func TestWriteFailureLearningState_RMWBranchIsByteIdenticalToTheGolden(t *testing.T) {
	const golden = `{"lastUpdated":"","lastCycleNumber":41,"version":0,"currentBatch":{"cycleAccruedCostUSD":0},"failedApproaches":[{"ts":"t40","cycle":40,"verdict":"FAIL","recordedAt":"r40","retrospected":true,"summary":"mine"},{"ts":"t9","cycle":9,"verdict":"FAIL","recordedAt":"r9","defects":["d"],"retrospected":false}],"carryoverTodos":[{"id":"peer","action":"peer todo","priority":"P1","first_seen_cycle":40,"cycles_unpicked":0},{"id":"shared","action":"disk","priority":"P0","first_seen_cycle":40,"cycles_unpicked":0,"expiresAt":"2026-01-01T00:00:00Z"},{"id":"new","action":"new todo","priority":"P1","first_seen_cycle":9,"cycles_unpicked":0,"expiresAt":"2026-03-01T00:00:00Z"}],"stateRevision":1}`
	f := &fakeUpdaterStorage{}
	f.mem.st = State{LastCycleNumber: 41,
		FailedAt:       []FailedRecord{{Cycle: 40, TS: "t40", Verdict: "FAIL", RecordedAt: "r40", Summary: "peer"}},
		CarryoverTodos: []CarryoverTodo{{ID: "peer", Action: "peer todo", Priority: "P1", FirstSeenCycle: 40}, {ID: "shared", Action: "disk", Priority: "P0", FirstSeenCycle: 40, ExpiresAt: "2026-01-01T00:00:00Z"}}}
	o := &Orchestrator{storage: f}
	o.writeFailureLearningState(context.Background(), &State{LastCycleNumber: 9,
		FailedAt:       []FailedRecord{{Cycle: 40, TS: "t40", Verdict: "FAIL", RecordedAt: "r40", Summary: "mine", Retrospected: true}, {Cycle: 9, TS: "t9", Verdict: "FAIL", RecordedAt: "r9", Defects: []string{"d"}}},
		CarryoverTodos: []CarryoverTodo{{ID: "shared", Action: "incoming", Priority: "P0", FirstSeenCycle: 9, ExpiresAt: "2026-02-01T00:00:00Z"}, {ID: "new", Action: "new todo", Priority: "P1", FirstSeenCycle: 9, ExpiresAt: "2026-03-01T00:00:00Z"}}})
	raw, err := json.Marshal(f.mem.st)
	if err != nil || string(raw) != golden {
		t.Fatalf("the merged state must marshal byte-identically to the pre-extraction golden:\n got %s\nwant %s (%v)", raw, golden, err)
	}
}

// The consumer pin of the unit's priority vocabulary: core's consts project
// the unit's (the advisor's rank table — the other consumer — pins the same
// vocabulary from its own package since ADR-0103 unit 04).
func TestCarryoverPriorityRank_RanksTheUnitsVocabulary(t *testing.T) {
	if carryoverPriorityBlocking != carryover.PriorityBlocking || carryoverPriorityLesson != carryover.PriorityLesson {
		t.Fatal("core's priority consts are projections of the unit's")
	}
}

// The lifecycle is exported now: every non-test construction outside the unit
// must be the orchestrator's wiredCarryover / nullCarryover, in ONE file — and
// the three Null-Object facades kept for tests and ACS predicates must have
// no production caller either (one would drop the unit's WARNs silently).
func TestCarryoverLifecycle_OneConstructionSite(t *testing.T) {
	const onlySite = "internal/core/carryover_lifecycle.go"
	for _, needle := range []string{"carryover.New(", "MergeWorkspaceCarryover(", "MergeWorkspacePrescriptionCarryover(", "ApplyDefectsAsCarryoverTodos("} {
		if offenders := nonTestSourcesMentioning(t, needle, onlySite); len(offenders) > 0 {
			t.Errorf("%q belongs to ONE non-test file (%s); these non-test files use it too: %v", needle, onlySite, offenders)
		}
	}
}

// nonTestSourcesMentioning lists the module's non-test Go files outside the
// carryover leaf and the one allowed site whose source contains needle.
func nonTestSourcesMentioning(t *testing.T, needle, allowed string) []string {
	t.Helper()
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var offenders []string
	walk := func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if entry.Name() == "vendor" || entry.Name() == "bin" || entry.Name() == "testdata" || (strings.HasPrefix(entry.Name(), ".") && path != moduleRoot) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || strings.HasPrefix(rel, "internal/core/carryover/") {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(body), needle) && rel != allowed {
			offenders = append(offenders, rel)
		}
		return nil
	}
	if err := filepath.WalkDir(moduleRoot, walk); err != nil {
		t.Fatal(err)
	}
	return offenders
}

// The cycle-1538 pin at the seam: through the REAL finalizeCycle, a memo that
// re-supplies a triage-dropped id cannot resurrect it (retire runs after both
// merges), and the prescription merge runs too.
func TestFinalizeCycle_CloseoutKeepsTheThreeStepOrder(t *testing.T) {
	workspace := t.TempDir()
	for name, body := range map[string]string{
		"triage-decision.json": `{"cycle":1538,"top_n":[],"dropped":[{"id":"stale-todo","reason":"stale"}]}`,
		"carryover-todos.json": `[{"id":"stale-todo","action":"the memo re-supplies it"},{"id":"memo-live","action":"keep me"}]`,
		"defect-ledger.json":   `{"origin_cycle":1538,"entries":[{"id":"rx-1","text":"PRESCRIPTION: enforce the guard","status":"OPEN"}]}`,
	} {
		if err := os.WriteFile(filepath.Join(workspace, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	storage := &fakeUpdaterStorage{}
	orchestrator := &Orchestrator{storage: storage, gitHEAD: func() (string, error) { return "same-head", nil }}
	state := &State{CarryoverTodos: []CarryoverTodo{{ID: "stale-todo", Action: "old copy"}}}
	result := &CycleResult{FinalVerdict: VerdictWARN}
	if _, err := orchestrator.finalizeCycle(context.Background(), CycleState{WorkspacePath: workspace}, 1538, "same-head", "", result, state, nil); err != nil {
		t.Fatalf("finalizeCycle: %v", err)
	}
	persisted := storage.mem.st.CarryoverTodos
	if carryoverTodoExists(persisted, "stale-todo") || !carryoverTodoExists(persisted, "memo-live") || !carryoverTodoExists(persisted, "rx-1") {
		t.Fatalf("memo merged, prescription merged, the dropped id retired AFTER both: %+v", persisted)
	}
}

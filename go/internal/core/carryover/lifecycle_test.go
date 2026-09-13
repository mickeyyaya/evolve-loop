package carryover

// lifecycle_test.go — unit 03: the Lifecycle's mint admission, the workspace
// readers, the closeout order, the persist, the Null Object and the codes.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func recordingSignals() (*signalcenter.Center, *[]signalcenter.Event) {
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	return c, got
}

func observed() (*Lifecycle, *[]signalcenter.Event) {
	c, got := recordingSignals()
	return New(WithSignals(func() *signalcenter.Center { return c })), got
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func oneWarn(t *testing.T, got []signalcenter.Event, code signalcenter.Code, origin string) signalcenter.Event {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("exactly one signal: %+v", got)
	}
	e := got[0]
	if e.Module != signalcenter.ModuleCarryover || e.Kind != signalcenter.KindCarryoverWarning || e.Severity != signalcenter.SeverityWarn || e.Code != code || e.Origin != origin {
		t.Fatalf("module carryover, kind carryover.warning, WARN, %s from %s: %+v", code, origin, e)
	}
	return e
}

func TestLifecycle_Append_FingerprintTwinRefreshesExpiryAndSuppresses(t *testing.T) {
	st := &cyclestate.State{CarryoverTodos: []cyclestate.CarryoverTodo{{ID: "cycle-1-failed-build", Action: "cycle 1 failed during build: x", ExpiresAt: "2026-01-01T00:00:00Z"}}}
	New().Append(st, cyclestate.CarryoverTodo{ID: "cycle-2-failed-build", Action: "cycle 2 failed during build: x", ExpiresAt: "2026-03-01T00:00:00Z"})
	if len(st.CarryoverTodos) != 1 || st.CarryoverTodos[0].ID != "cycle-1-failed-build" || st.CarryoverTodos[0].ExpiresAt != "2026-03-01T00:00:00Z" {
		t.Fatalf("the survivor keeps its ID and takes the later stamp; nothing is appended: %+v", st.CarryoverTodos)
	}
}

func TestLifecycle_Append_FingerprintTwinWithOlderStampIsNotRefreshed(t *testing.T) {
	st := &cyclestate.State{CarryoverTodos: []cyclestate.CarryoverTodo{{ID: "a", Action: "cycle 1 failed during build: x", ExpiresAt: "2026-03-01T00:00:00Z"}}}
	New().Append(st, cyclestate.CarryoverTodo{ID: "b", Action: "cycle 2 failed during build: x", ExpiresAt: "2026-01-01T00:00:00Z"})
	if len(st.CarryoverTodos) != 1 || st.CarryoverTodos[0].ExpiresAt != "2026-03-01T00:00:00Z" {
		t.Fatalf("an older stamp leaves the survivor untouched: %+v", st.CarryoverTodos)
	}
}

func TestLifecycle_Append_IDTwinIsSkippedNilStateInertElseAppendsLast(t *testing.T) {
	New().Append(nil, cyclestate.CarryoverTodo{ID: "a"}) // inert
	st := &cyclestate.State{CarryoverTodos: []cyclestate.CarryoverTodo{{ID: "a", Action: "one"}}}
	New().Append(st, cyclestate.CarryoverTodo{ID: "a", Action: "two"})
	if len(st.CarryoverTodos) != 1 || st.CarryoverTodos[0].Action != "one" {
		t.Fatalf("an id twin with a different action is skipped: %+v", st.CarryoverTodos)
	}
	New().Append(st, cyclestate.CarryoverTodo{ID: "b", Action: "three"})
	if len(st.CarryoverTodos) != 2 || st.CarryoverTodos[1].ID != "b" {
		t.Fatalf("a new todo appends last: %+v", st.CarryoverTodos)
	}
}

func TestLifecycle_ApplyDefects_MintsOneTodoPerNonBlankDefect(t *testing.T) {
	st := &cyclestate.State{}
	long := strings.Repeat("d", 600)
	New().ApplyDefects(st, cyclestate.FailedRecord{Cycle: 7, ExpiresAt: "2026-05-01T00:00:00Z", Defects: []string{"first", "  ", long}})
	if len(st.CarryoverTodos) != 2 {
		t.Fatalf("one todo per non-blank defect: %+v", st.CarryoverTodos)
	}
	a, b := st.CarryoverTodos[0], st.CarryoverTodos[1]
	if a.ID != "cycle-7-defect-0" || b.ID != "cycle-7-defect-1" || a.Action != "Fix defect from cycle 7: first" || b.Action != "Fix defect from cycle 7: "+strings.Repeat("d", 500)+"…" {
		t.Fatalf("ids count non-blank only; the action is capped at MaxActionRunes: %+v", st.CarryoverTodos)
	}
	if a.Priority != PriorityBlocking || a.FirstSeenCycle != 7 || a.CyclesUnpicked != 0 || a.ExpiresAt != "2026-05-01T00:00:00Z" {
		t.Fatalf("P0, first seen this cycle, the record's stamp inherited: %+v", a)
	}
	empty := &cyclestate.State{}
	New().ApplyDefects(empty, cyclestate.FailedRecord{Cycle: 8, Defects: []string{"x"}})
	if empty.CarryoverTodos[0].ExpiresAt != "" {
		t.Fatal("an unstamped record leaves the todo unstamped")
	}
	New().ApplyDefects(empty, cyclestate.FailedRecord{Cycle: 9})
	if len(empty.CarryoverTodos) != 1 {
		t.Fatal("nil defects mint nothing")
	}
}

func TestLifecycle_ApplyDefects_CrossCycleTwinRefreshesExpiryAndIsIdempotent(t *testing.T) {
	st := &cyclestate.State{}
	l := New()
	l.ApplyDefects(st, cyclestate.FailedRecord{Cycle: 1424, ExpiresAt: "2026-01-01T00:00:00Z", Defects: []string{"missing test"}})
	l.ApplyDefects(st, cyclestate.FailedRecord{Cycle: 1427, ExpiresAt: "2026-02-01T00:00:00Z", Defects: []string{"missing test"}})
	l.ApplyDefects(st, cyclestate.FailedRecord{Cycle: 1427, ExpiresAt: "2026-02-01T00:00:00Z", Defects: []string{"missing test"}})
	if len(st.CarryoverTodos) != 1 || st.CarryoverTodos[0].ID != "cycle-1424-defect-0" || st.CarryoverTodos[0].ExpiresAt != "2026-02-01T00:00:00Z" {
		t.Fatalf("a cross-cycle twin refreshes the survivor's expiry and mints nothing; re-applying is idempotent: %+v", st.CarryoverTodos)
	}
}

func TestLifecycle_MergeMemo_AbsentFileIsSilentAndANoOp(t *testing.T) {
	l, got := observed()
	st := &cyclestate.State{}
	l.MergeMemo(st, t.TempDir(), 5, time.Now())
	l.MergeMemo(nil, t.TempDir(), 5, time.Now())
	l.MergeMemo(st, "  ", 5, time.Now())
	if len(*got) != 0 || len(st.CarryoverTodos) != 0 {
		t.Fatalf("absent file, nil state, blank workspace: silent no-ops: %+v %+v", *got, st.CarryoverTodos)
	}
}

func TestLifecycle_MergeMemo_ReadErrorIsAWarnSignal(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "carryover-todos.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	l, got := observed()
	st := &cyclestate.State{}
	l.MergeMemo(st, ws, 5, time.Now())
	e := oneWarn(t, *got, CodeWorkspaceReadFailed, "Lifecycle.MergeMemo")
	if e.Cycle != 5 || e.Fields["document"] != "carryover-todos.json" || e.Fields["path"] != filepath.Join(ws, "carryover-todos.json") || !strings.Contains(e.Reason, "read failed") {
		t.Fatalf("a read fault that is not absence is a WARN naming the document and path: %+v", e)
	}
	if len(st.CarryoverTodos) != 0 {
		t.Fatal("nothing merged")
	}
}

func TestLifecycle_MergeMemo_MalformedIsAWarnSignalAndMergesNothing(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "carryover-todos.json", "{not json")
	l, got := observed()
	st := &cyclestate.State{}
	l.MergeMemo(st, ws, 5, time.Now())
	e := oneWarn(t, *got, CodeWorkspaceMalformed, "Lifecycle.MergeMemo")
	if !strings.Contains(e.Reason, "malformed") || len(st.CarryoverTodos) != 0 {
		t.Fatalf("the file is skipped whole with a WARN carrying the decode error: %+v", e)
	}
}

func TestLifecycle_MergeMemo_DecodesCapsDefaultsPriorityAndStampsDefaultExpiry(t *testing.T) {
	ws := t.TempDir()
	long := strings.Repeat("a", 600)
	writeFile(t, ws, "carryover-todos.json", `[{"id":" m1 ","action":" do it ","priority":"","evidence_pointer":"x"},{"id":"","action":"skipped"},{"id":"m2","action":"   "},{"id":"m3","action":"`+long+`","priority":"high"}]`)
	l, got := observed()
	st := &cyclestate.State{CarryoverTodos: []cyclestate.CarryoverTodo{{ID: "m3", Action: "already here"}}}
	now := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	l.MergeMemo(st, ws, 42, now)
	if len(*got) != 0 {
		t.Fatalf("a clean merge is silent in commit 1: %+v", *got)
	}
	if len(st.CarryoverTodos) != 2 || st.CarryoverTodos[1].ID != "m1" || st.CarryoverTodos[1].Action != "do it" || st.CarryoverTodos[1].Priority != PriorityMemoDefault || st.CarryoverTodos[1].FirstSeenCycle != 42 || st.CarryoverTodos[1].ExpiresAt != "2026-08-10T00:00:00Z" {
		t.Fatalf("trimmed, defaulted to medium, stamped now+30d, id/action-less skipped, dedupe by id keeps the disk copy: %+v", st.CarryoverTodos)
	}
	if st.CarryoverTodos[0].Action != "already here" {
		t.Fatal("the disk copy of a shared id survives (ID-only union)")
	}
}

func TestLifecycle_MergeMemo_CapsTheAction(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "carryover-todos.json", `[{"id":"m","action":"`+strings.Repeat("a", 600)+`"}]`)
	st := &cyclestate.State{}
	New().MergeMemo(st, ws, 1, time.Now())
	if len(st.CarryoverTodos) != 1 || st.CarryoverTodos[0].Action != strings.Repeat("a", 500)+"…" {
		t.Fatalf("the action is capped at MaxActionRunes: %d", len([]rune(st.CarryoverTodos[0].Action)))
	}
}

func TestLifecycle_MergePrescriptions_KeepsOnlyOpenPrefixedRows(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "defect-ledger.json", `{"origin_cycle":40,"entries":[{"id":"p1","text":"PRESCRIPTION: add the guard","status":" OPEN "},{"id":"p2","text":"PRESCRIPTION: fixed one","status":"FIXED"},{"id":"p3","text":"ordinary defect","status":"OPEN"},{"id":"","text":"PRESCRIPTION: no id","status":"OPEN"},{"id":"p5","text":"PRESCRIPTION: deferred","status":"DEFERRED"}]}`)
	l, got := observed()
	st := &cyclestate.State{}
	now := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	l.MergePrescriptions(st, ws, 42, now)
	if len(*got) != 0 || len(st.CarryoverTodos) != 1 {
		t.Fatalf("only the OPEN prefixed row is carried: %+v %+v", *got, st.CarryoverTodos)
	}
	p := st.CarryoverTodos[0]
	if p.ID != "p1" || p.Action != "PRESCRIPTION: add the guard" || p.Priority != PriorityPrescription || p.FirstSeenCycle != 42 || p.ExpiresAt != "2026-08-10T00:00:00Z" {
		t.Fatalf("the prefix stays inside the action, priority high, stamped now+30d: %+v", p)
	}
	l.MergePrescriptions(st, t.TempDir(), 43, now) // absent: silent
	if len(*got) != 0 {
		t.Fatal("absent ledger is silent")
	}
	writeFile(t, ws, "defect-ledger.json", "[oops")
	l.MergePrescriptions(st, ws, 43, now)
	e := oneWarn(t, *got, CodeWorkspaceMalformed, "Lifecycle.MergePrescriptions")
	if e.Fields["document"] != "defect-ledger.json" {
		t.Fatalf("the malformed WARN names the ledger: %+v", e.Fields)
	}
}

func TestLifecycle_MergePrescriptions_ReadErrorIsAWarnSignal(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "defect-ledger.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	l, got := observed()
	l.MergePrescriptions(&cyclestate.State{}, ws, 1, time.Now())
	oneWarn(t, *got, CodeWorkspaceReadFailed, "Lifecycle.MergePrescriptions")
}

func TestLifecycle_RetireTriageDropped_DeferredSurvives(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "triage-decision.json", `{"top_n":[],"deferred":[{"id":"keep","reason":"later"}],"dropped":[{"id":"gone","reason":"stale"},{"id":"  "}]}`)
	st := &cyclestate.State{CarryoverTodos: []cyclestate.CarryoverTodo{{ID: "gone"}, {ID: "keep"}, {ID: "other"}}}
	l, got := observed()
	l.RetireTriageDropped(st, ws)
	if len(*got) != 0 || len(st.CarryoverTodos) != 2 || st.CarryoverTodos[0].ID != "keep" || st.CarryoverTodos[1].ID != "other" {
		t.Fatalf("only the dropped id retires; deferred survives; order kept; silent in commit 1: %+v %+v", *got, st.CarryoverTodos)
	}
}

func TestLifecycle_RetireTriageDropped_AbsentOrMalformedDecisionRetiresNothingSilently(t *testing.T) {
	st := &cyclestate.State{CarryoverTodos: []cyclestate.CarryoverTodo{{ID: "a"}}}
	l, got := observed()
	l.RetireTriageDropped(st, t.TempDir())
	ws := t.TempDir()
	writeFile(t, ws, "triage-decision.json", "{broken")
	l.RetireTriageDropped(st, ws)
	writeFile(t, ws, "triage-decision.json", `{"dropped":[]}`)
	l.RetireTriageDropped(st, ws)
	if len(*got) != 0 || len(st.CarryoverTodos) != 1 {
		t.Fatalf("absent, malformed and empty decisions retire nothing, silently (the preserved silence; F7): %+v %+v", *got, st.CarryoverTodos)
	}
}

func TestLifecycle_RetireTriageDropped_NilEmptyAndBlankWorkspaceAreInert(t *testing.T) {
	l := New()
	l.RetireTriageDropped(nil, t.TempDir())
	empty := &cyclestate.State{}
	l.RetireTriageDropped(empty, t.TempDir())
	st := &cyclestate.State{CarryoverTodos: []cyclestate.CarryoverTodo{{ID: "a"}}}
	l.RetireTriageDropped(st, "")
	if len(st.CarryoverTodos) != 1 {
		t.Fatal("a blank workspace retires nothing")
	}
}

func TestLifecycle_Closeout_RetiresAfterBothMerges(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "carryover-todos.json", `[{"id":"dropped-by-triage","action":"memo re-supplies it"},{"id":"shared","action":"memo copy"}]`)
	writeFile(t, ws, "defect-ledger.json", `{"entries":[{"id":"shared","text":"PRESCRIPTION: ledger copy","status":"OPEN"},{"id":"p","text":"PRESCRIPTION: keep","status":"OPEN"}]}`)
	writeFile(t, ws, "triage-decision.json", `{"dropped":[{"id":"dropped-by-triage"}]}`)
	st := &cyclestate.State{}
	New().Closeout(st, ws, 9, time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC))
	if HasID(st.CarryoverTodos, "dropped-by-triage") {
		t.Fatalf("retire runs AFTER both merges, so a memo cannot resurrect a dropped id (cycle 1538): %+v", st.CarryoverTodos)
	}
	if len(st.CarryoverTodos) != 2 || st.CarryoverTodos[0].ID != "shared" || st.CarryoverTodos[0].Action != "memo copy" || st.CarryoverTodos[1].ID != "p" {
		t.Fatalf("memo first, then prescriptions (disk-first on a shared id): %+v", st.CarryoverTodos)
	}
}

type panickyStore struct{}

func (panickyStore) WriteState(context.Context, cyclestate.State) error { panic("never called") }

type legacyStore struct {
	fail bool
	got  []cyclestate.State
}

func (s *legacyStore) WriteState(_ context.Context, st cyclestate.State) error {
	if s.fail {
		return errors.New("forced WriteState fail")
	}
	s.got = append(s.got, st)
	return nil
}

type fakeUpdater struct {
	legacyStore
	st       cyclestate.State
	revision int
	fail     bool
}

func (u *fakeUpdater) UpdateState(_ context.Context, mutate func(*cyclestate.State)) (cyclestate.State, error) {
	if u.fail {
		return cyclestate.State{}, errors.New("forced UpdateState fail")
	}
	mutate(&u.st)
	u.revision++
	return u.st, nil
}

var (
	_ Store   = (*legacyStore)(nil)
	_ Updater = (*fakeUpdater)(nil)
)

func TestLifecycle_Persist_NilStateIsInert(t *testing.T) {
	New().Persist(context.Background(), panickyStore{}, nil)
}

func TestLifecycle_Persist_NilStorePanicsRatherThanDroppingTheState(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a nil store must panic: silently dropping the failure-learning state would be the worst failure mode")
		}
	}()
	New().Persist(context.Background(), nil, &cyclestate.State{LastCycleNumber: 1})
}

func TestLifecycle_Persist_LegacyStoreWritesTheWholeState(t *testing.T) {
	l, got := observed()
	store := &legacyStore{}
	st := &cyclestate.State{LastCycleNumber: 9, CarryoverTodos: []cyclestate.CarryoverTodo{{ID: "a"}}}
	l.Persist(context.Background(), store, st)
	if len(store.got) != 1 || store.got[0].LastCycleNumber != 9 || len(store.got[0].CarryoverTodos) != 1 || len(*got) != 0 {
		t.Fatalf("a store without an Updater receives the whole state verbatim, silently: %+v %+v", store.got, *got)
	}
	failing := &legacyStore{fail: true}
	l.Persist(context.Background(), failing, st)
	e := oneWarn(t, *got, CodePersistFailed, "Lifecycle.Persist")
	if !strings.HasPrefix(e.Reason, "state write failed: ") || e.Fields["step"] != "write" || e.Cycle != 9 || e.Fields["last_cycle"] != "" {
		t.Fatalf("the WARN names the step and the error and carries the cycle ONCE, on Event.Cycle: %+v", e)
	}
}

func TestLifecycle_Persist_UpdaterMergesOnlyTheTwoArrays(t *testing.T) {
	l, got := observed()
	u := &fakeUpdater{st: cyclestate.State{LastCycleNumber: 41,
		FailedAt:       []cyclestate.FailedRecord{{Cycle: 40, TS: "t40", Verdict: "FAIL", RecordedAt: "r40", Summary: "peer"}},
		CarryoverTodos: []cyclestate.CarryoverTodo{{ID: "peer", Action: "peer todo"}, {ID: "shared", Action: "disk", ExpiresAt: "2026-01-01T00:00:00Z"}}}}
	st := &cyclestate.State{LastCycleNumber: 9,
		FailedAt:       []cyclestate.FailedRecord{{Cycle: 40, TS: "t40", Verdict: "FAIL", RecordedAt: "r40", Summary: "mine"}, {Cycle: 9, TS: "t9", Verdict: "FAIL", RecordedAt: "r9"}},
		CarryoverTodos: []cyclestate.CarryoverTodo{{ID: "shared", Action: "incoming", ExpiresAt: "2026-02-01T00:00:00Z"}, {ID: "new"}}}
	l.Persist(context.Background(), u, st)
	if len(*got) != 0 || u.revision != 1 || len(u.legacyStore.got) != 0 {
		t.Fatalf("the RMW branch is taken, once, silently: %+v rev=%d legacy=%d", *got, u.revision, len(u.legacyStore.got))
	}
	if u.st.LastCycleNumber != 41 {
		t.Fatalf("LastCycleNumber is NOT merged by this seam (Q2, preserved): %d", u.st.LastCycleNumber)
	}
	if len(u.st.FailedAt) != 2 || u.st.FailedAt[0].Summary != "mine" || u.st.FailedAt[1].Cycle != 9 {
		t.Fatalf("records: incoming wins on a shared key, disk order kept: %+v", u.st.FailedAt)
	}
	if len(u.st.CarryoverTodos) != 3 || u.st.CarryoverTodos[1].Action != "disk" || u.st.CarryoverTodos[2].ID != "new" {
		t.Fatalf("todos: disk-first, ID-only, the disk copy kept (Q3, preserved): %+v", u.st.CarryoverTodos)
	}
	failing := &fakeUpdater{fail: true}
	l.Persist(context.Background(), failing, st)
	e := oneWarn(t, *got, CodePersistFailed, "Lifecycle.Persist")
	if !strings.HasPrefix(e.Reason, "state update failed: ") || e.Fields["step"] != "update" || e.Cycle != 9 {
		t.Fatalf("the RMW failure names its step and the cycle: %+v", e)
	}
}

func TestLifecycle_Persist_MergedStateMarshalsByteIdentical(t *testing.T) {
	u := &fakeUpdater{}
	st := &cyclestate.State{
		FailedAt:       []cyclestate.FailedRecord{{TS: "t", Cycle: 3, Verdict: "FAIL", Classification: "c", RecordedAt: "r", ExpiresAt: "e", Defects: []string{"d"}, Retrospected: true, Summary: "s"}},
		CarryoverTodos: []cyclestate.CarryoverTodo{{ID: "i", Action: "a", Priority: "P0", FirstSeenCycle: 3, ExpiresAt: "x"}},
	}
	New().Persist(context.Background(), u, st)
	raw, err := json.Marshal(u.st)
	if err != nil {
		t.Fatal(err)
	}
	const wantTodos = `"carryoverTodos":[{"id":"i","action":"a","priority":"P0","first_seen_cycle":3,"cycles_unpicked":0,"expiresAt":"x"}]`
	const wantRecords = `"failedApproaches":[{"ts":"t","cycle":3,"verdict":"FAIL","classification":"c","recordedAt":"r","expiresAt":"e","defects":["d"],"retrospected":true,"summary":"s"}]`
	if !strings.Contains(string(raw), wantTodos) || !strings.Contains(string(raw), wantRecords) {
		t.Fatalf("the wire order is cyclestate's, untouched by the unit:\n%s", raw)
	}
}

func TestWithSignals_NilIsTheNullObject(t *testing.T) {
	opts := []Option{WithSignals(nil)}
	l := New(opts...)
	if l.SignalsWired() {
		t.Fatal("nil is reported unwired")
	}
	if w, _ := observed(); !w.SignalsWired() {
		t.Fatal("a Center is reported wired")
	}
	ws := t.TempDir()
	writeFile(t, ws, "carryover-todos.json", "{broken")
	st := &cyclestate.State{}
	l.MergeMemo(st, ws, 1, time.Now()) // the malformed path with no Center must not panic
	if len(st.CarryoverTodos) != 0 {
		t.Fatal("skipped")
	}
}

func TestWithSignals_ReadsTheCenterLive(t *testing.T) {
	var late *signalcenter.Center
	l := New(WithSignals(func() *signalcenter.Center { return late }))
	if l.SignalsWired() {
		t.Fatal("no Center yet: unwired")
	}
	var got []signalcenter.Event
	late = signalcenter.New()
	late.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
	if !l.SignalsWired() {
		t.Fatal("the Center installed after construction is seen")
	}
	l.Persist(context.Background(), &legacyStore{fail: true}, &cyclestate.State{})
	if len(got) != 1 || got[0].Code != CodePersistFailed {
		t.Fatalf("the late Center receives the unit's signal: %+v", got)
	}
}

func TestCarryoverCodes_AreRegisteredWithDocs(t *testing.T) {
	for _, code := range []signalcenter.Code{CodeWorkspaceReadFailed, CodeWorkspaceMalformed, CodePersistFailed} {
		if m, ok := signalcenter.IsRegistered(code); !ok || m != signalcenter.ModuleCarryover {
			t.Fatalf("%s is registered under module carryover: %v %v", code, m, ok)
		}
	}
}

// The workspace merges are ID-only unions (no fingerprint suppression — the
// critic's fold): a memo entry whose action fingerprints an existing P0 todo
// is still appended, exactly as carryover_merge.go did.
func TestLifecycle_MergeMemo_IsAnIDOnlyUnionNeverFingerprintSuppressed(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "carryover-todos.json", `[{"id":"memo-1","action":"cycle 2 failed during build: x"}]`)
	st := &cyclestate.State{CarryoverTodos: []cyclestate.CarryoverTodo{{ID: "cycle-1-failed-build", Action: "cycle 1 failed during build: x"}}}
	New().MergeMemo(st, ws, 2, time.Now())
	if len(st.CarryoverTodos) != 2 || st.CarryoverTodos[1].ID != "memo-1" {
		t.Fatalf("a fingerprint twin from the memo is appended (ID-only union): %+v", st.CarryoverTodos)
	}
}

func TestLifecycle_MergePrescriptions_NilStateAndBlankWorkspaceAreInert(t *testing.T) {
	l, got := observed()
	l.MergePrescriptions(nil, t.TempDir(), 1, time.Now())
	st := &cyclestate.State{}
	l.MergePrescriptions(st, " ", 1, time.Now())
	if len(*got) != 0 || len(st.CarryoverTodos) != 0 {
		t.Fatal("inert")
	}
}

// ApplyDefects keeps the pre-extraction contract on a nil state — a panic,
// not a silent no-op (only Append guards nil); pinned so the ONE admission
// rule cannot quietly widen it.
func TestLifecycle_ApplyDefects_NilStatePanicsAsBefore(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("ApplyDefects(nil, record) panicked before the extraction and must still")
		}
	}()
	New().ApplyDefects(nil, cyclestate.FailedRecord{Cycle: 1, Defects: []string{"x"}})
}

package failurelearning

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var fixedNow = time.Date(2026, 7, 15, 1, 2, 3, 0, time.UTC)

func clock() time.Time { return fixedNow }

// observed builds an engine reporting into a recording Center.
func observed(t *testing.T) (*Engine, *[]signalcenter.Event) {
	t.Helper()
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	e := New(clock, carryover.New(), WithSignals(func() *signalcenter.Center { return c }))
	return e, got
}

// seedBlock writes a phase report carrying a structured failure block.
func seedBlock(t *testing.T, ws, phase string, fb *phasecontract.FailureBlock) {
	t.Helper()
	body := "## " + strings.ToUpper(phase[:1]) + phase[1:] + "\nFAIL\n" + phasecontract.RenderVerdictSentinelWithFailure(phase, "FAIL", fb) + "\n"
	if err := os.WriteFile(filepath.Join(ws, phase+"-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func auditFailure(root, ws string) Failure {
	return Failure{Cycle: 1279, Phase: cyclestate.PhaseAudit, Err: errors.New("audit phase exited 1 after 3 attempts"), ProjectRoot: root, Workspace: ws}
}

// Test 14 — the recorder mints the FailedRecord and the P0 todo over the SAME
// state, stamps LastCycleNumber, and returns what it derived.
func TestRecordFailedApproach_MintsRecordAndTodoAndStampsState(t *testing.T) {
	e := New(clock, carryover.New())
	st := &cyclestate.State{LastCycleNumber: 1278}
	f := auditFailure(t.TempDir(), t.TempDir())
	l := e.RecordFailedApproach(st, f)
	summary := "cycle 1279 failed during audit: audit phase exited 1 after 3 attempts"
	if l.Summary != summary || l.TodoID != "cycle-1279-failed-audit" || l.Structured != nil {
		t.Fatalf("Learned: %+v", l)
	}
	wantExpiry := failurelog.ComputeExpiresAt(failurelog.NormalizeLegacy(cyclestate.ClassificationMidExecutionFail), fixedNow)
	if len(st.FailedAt) != 1 {
		t.Fatalf("one record: %+v", st.FailedAt)
	}
	r := st.FailedAt[0]
	if r.TS != "2026-07-15T01:02:03Z" || r.RecordedAt != r.TS || r.Cycle != 1279 || r.Verdict != cyclestate.VerdictFAIL ||
		r.Classification != cyclestate.ClassificationMidExecutionFail || r.Summary != summary || len(r.Defects) != 1 || r.Defects[0] != summary ||
		!r.Retrospected || r.ExpiresAt != wantExpiry {
		t.Fatalf("the record verbatim: %+v (want expiry %s)", r, wantExpiry)
	}
	if len(st.CarryoverTodos) != 1 {
		t.Fatalf("one todo: %+v", st.CarryoverTodos)
	}
	td := st.CarryoverTodos[0]
	if td.ID != "cycle-1279-failed-audit" || td.Action != summary || td.Priority != carryover.PriorityBlocking || td.FirstSeenCycle != 1279 || td.ExpiresAt != r.ExpiresAt {
		t.Fatalf("the todo inherits the record's stamp: %+v", td)
	}
	if st.LastCycleNumber != 1279 {
		t.Fatalf("LastCycleNumber stamped: %d", st.LastCycleNumber)
	}
}

// Test 15 — the State's wire shape is byte-identical to the golden captured
// on 97825125 (a seeded structured block, the fixed clock).
func TestRecordFailedApproach_MarshalsByteIdenticalToTheGolden(t *testing.T) {
	ws := t.TempDir()
	seedBlock(t, ws, "audit", &phasecontract.FailureBlock{Class: "code-build-fail", Defects: []string{"d1", "d2"}, EvidencePaths: []string{"go/x_test.go"}})
	e := New(clock, carryover.New())
	st := &cyclestate.State{LastCycleNumber: 1278}
	l := e.RecordFailedApproach(st, auditFailure(t.TempDir(), ws))
	got, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "record-state.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("state.json bytes drifted from the pre-extraction golden:\n got %s\nwant %s", got, want)
	}
	if l.Structured == nil || l.Structured.Class != "code-build-fail" {
		t.Fatalf("the adopted block is returned: %+v", l.Structured)
	}
}

func TestRecordFailedApproach_AdoptsTheStructuredBlockAndItsTTL(t *testing.T) {
	ws := t.TempDir()
	seedBlock(t, ws, "audit", &phasecontract.FailureBlock{Class: "code-build-fail"})
	e := New(clock, carryover.New())
	st := &cyclestate.State{}
	l := e.RecordFailedApproach(st, auditFailure(t.TempDir(), ws))
	r := st.FailedAt[0]
	if r.Classification != "code-build-fail" || len(r.Defects) != 1 || r.Defects[0] != l.Summary {
		t.Fatalf("a classed block with no defects keeps the summary as the one defect: %+v", r)
	}
	if want := failurelog.ComputeExpiresAt(failurelog.NormalizeLegacy("code-build-fail"), fixedNow); r.ExpiresAt != want || st.CarryoverTodos[0].ExpiresAt != want {
		t.Fatalf("the TTL follows the ADOPTED class: %s / %s, want %s", r.ExpiresAt, st.CarryoverTodos[0].ExpiresAt, want)
	}
}

func TestRecordFailedApproach_DedupesTheTodoThroughTheLifecycle(t *testing.T) {
	e := New(clock, carryover.New())
	st := &cyclestate.State{}
	f := auditFailure(t.TempDir(), t.TempDir())
	e.RecordFailedApproach(st, f)
	e.RecordFailedApproach(st, f)
	if len(st.FailedAt) != 2 || len(st.CarryoverTodos) != 1 {
		t.Fatalf("two records, ONE todo (the id twin is skipped by the lifecycle): %d / %d", len(st.FailedAt), len(st.CarryoverTodos))
	}
}

func TestRecordFailedApproach_NilStatePanicsAsBefore(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a nil state is a programming error (callers guarantee it) — no guard, no silent skip")
		}
	}()
	New(clock, carryover.New()).RecordFailedApproach(nil, auditFailure("", ""))
}

// Test 16 — the ADR-0039 §7 trust boundary (moved verbatim from core).
func TestAdoptStructuredFailure_TrustBoundary(t *testing.T) {
	ws := t.TempDir()
	seedBlock(t, ws, "triage", &phasecontract.FailureBlock{Class: "totally-novel-class", Defects: []string{"d"}})
	if fb := adoptStructuredFailure(ws, "triage"); fb != nil {
		t.Errorf("out-of-taxonomy class must be refused; got %+v", fb)
	}
	big := strings.Repeat("x", 2000)
	many := make([]string, 50)
	for i := range many {
		many[i] = big
	}
	seedBlock(t, ws, "triage", &phasecontract.FailureBlock{Class: "code-build-fail", Defects: many})
	fb := adoptStructuredFailure(ws, "triage")
	if fb == nil {
		t.Fatal("canonical class must be adopted")
	}
	if len(fb.Defects) != maxAdoptedDefects {
		t.Errorf("defect list not capped: %d", len(fb.Defects))
	}
	if r := []rune(fb.Defects[0]); len(r) != maxAdoptedDefectRunes+1 { // +1 for the ellipsis
		t.Errorf("defect entry not capped: %d runes", len(r))
	}
	if adoptStructuredFailure(t.TempDir(), "triage") != nil {
		t.Error("no report, no block")
	}
}

func TestCapStrings_BoundsEntriesAndRunesWithTheCarryoverCap(t *testing.T) {
	in := []string{strings.Repeat("a", 6), "b", "c", "d"}
	got := capStrings(in, 3, 5)
	if len(got) != 3 || got[0] != carryover.CapRunes(in[0], 5) || got[1] != "b" {
		t.Fatalf("entries capped at 3, runes through carryover.CapRunes: %q", got)
	}
	if got := capStrings([]string{"ab"}, 3, 2); got[0] != "ab" {
		t.Fatalf("at the rune bound: untouched, got %q", got[0])
	}
}

// Test 27 — construction contract: options, the live accessor, the Null
// Object, the registered codes. Every export is named here (apicover).
func TestWithSignals_NilIsTheNullObject(t *testing.T) {
	for _, opts := range [][]Option{nil, {WithSignals(nil)}, {WithSignals(func() *signalcenter.Center { return nil })}} {
		e := New(clock, carryover.New(), opts...)
		if e.SignalsWired() {
			t.Fatal("no Center ⇒ not wired")
		}
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".evolve", "policy.json"), 0o755); err != nil {
			t.Fatal(err)
		}
		e.WriteFloor(Failure{Cycle: 1, Phase: cyclestate.PhaseAudit, ProjectRoot: root, Workspace: t.TempDir()}, Learned{Summary: "s"}) // a provoked fault emits into nothing
	}
}

func TestWithSignals_ReadsTheCenterLive(t *testing.T) {
	var c *signalcenter.Center
	e := New(clock, carryover.New(), WithSignals(func() *signalcenter.Center { return c }))
	if e.SignalsWired() {
		t.Fatal("not wired before the Center exists")
	}
	c = signalcenter.New()
	var got []signalcenter.Event
	c.Subscribe(func(ev signalcenter.Event) { got = append(got, ev) })
	if !e.SignalsWired() {
		t.Fatal("the accessor is read live, never snapshotted at construction")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "policy.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	e.WriteFloor(Failure{Cycle: 7, Phase: cyclestate.PhaseBuild, ProjectRoot: root, Workspace: t.TempDir()}, Learned{Summary: "s"})
	if len(got) != 1 || got[0].Code != CodePolicyLoadFailed {
		t.Fatalf("the Center applied after construction receives the WARN: %+v", got)
	}
}

func TestFailureLearningCodes_AreRegisteredWithDocs(t *testing.T) {
	for _, c := range []signalcenter.Code{CodePolicyLoadFailed, CodeFloorWriteFailed, CodeRemediationTruncated, CodeRecurrenceLedgerFailed} {
		if m, ok := signalcenter.IsRegistered(c); !ok || m != signalcenter.ModuleFailureLearning {
			t.Fatalf("%s must be registered under module failurelearning (got %q, %v)", c, m, ok)
		}
	}
}

// The two request shapes are constructed positionally so a new field breaks
// this test at compile time and the ONE projection (core's failureOf) and the
// floor-verdict producer are revisited (the 11-field DTO's silent-zero hazard).
func TestRequestShapes_HaveExactlyTheDeclaredFields(t *testing.T) {
	f := Failure{1, cyclestate.PhaseAudit, errors.New("e"), "root", "ws"}
	l := Learned{"summary", "todo", &phasecontract.FailureBlock{}}
	if f.Cycle != 1 || l.TodoID != "todo" {
		t.Fatal("positional construction")
	}
}

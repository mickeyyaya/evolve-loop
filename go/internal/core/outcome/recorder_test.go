package outcome

// recorder_test.go — unit 01 (ADR-0103, design decomposition/01-outcome-recorder.md):
// the C1 recording chokepoint as its own small unit. Every terminal phase
// disposition is recorded exactly once — into the cycle result, the
// phase-timing log and the <phase>-usage.json sidecar — with EndedAt, the
// archetype and the context fill stamped here, the orchestrator's emission
// hook called at the same point as before, and the unit's own failure modes
// raised as outcome.warning signals under module outcome.

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var fixedNow = func() time.Time { return time.Date(2026, 9, 13, 1, 2, 3, 0, time.UTC) }

func archetypeOf(phase string) string { return "role-" + phase }

type emitted struct {
	cycle int
	out   recovery.PhaseOutcome
}

func recordingSignals() (*signalcenter.Center, *[]signalcenter.Event) {
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	return c, got
}

func newTestRecorder(c *signalcenter.Center) (*Recorder, *[]emitted) {
	calls := &[]emitted{}
	r := NewRecorder(fixedNow, archetypeOf, func(cycle int, out recovery.PhaseOutcome) {
		*calls = append(*calls, emitted{cycle, out})
	}, WithSignals(func() *signalcenter.Center { return c }))
	return r, calls
}

func scoutOutcome() recovery.PhaseOutcome {
	return recovery.PhaseOutcome{Phase: "scout", Verdict: "PASS", CostUSD: 0.5, DurationMS: 1200, BootMS: 40, StartedAt: "2026-09-13T01:00:00Z", AttemptCount: 1, ModelSource: "profile", ResolvedModel: "unknown-tier"}
}

func TestRecorder_Record_AppendsThePhaseAndTheTimingEntry(t *testing.T) {
	c, _ := recordingSignals()
	r, _ := newTestRecorder(c)
	result := &cyclestate.CycleResult{Cycle: 7, PhasesRun: []cyclestate.Phase{"intent"}}
	timings := []phasetiming.Entry{{Phase: "intent"}}
	r.Record(result, &timings, t.TempDir(), scoutOutcome())
	if len(result.PhasesRun) != 2 || result.PhasesRun[1] != "scout" {
		t.Fatalf("the phase is appended to PhasesRun in order: %v", result.PhasesRun)
	}
	if len(timings) != 2 {
		t.Fatalf("the timing entry is appended in order: %+v", timings)
	}
	e := timings[1]
	if e.Phase != "scout" || e.Verdict != "PASS" || e.CostUSD != 0.5 || e.DurationMS != 1200 || e.BootMS != 40 || e.StartedAt != "2026-09-13T01:00:00Z" ||
		e.EndedAt != "2026-09-13T01:02:03Z" || e.Archetype != "role-scout" || e.AttemptCount != 1 || e.ModelSource != "profile" || e.ResolvedModel != "unknown-tier" {
		t.Errorf("the entry carries the outcome with EndedAt from the clock and Archetype from the lookup: %+v", e)
	}
	if e.ContextFillRatio != 0 || e.ContextWindowHot {
		t.Errorf("an unknown tier records an absent fill, never a guess: %+v", e)
	}
}

func TestRecorder_Record_CallsEmitOnceAfterTheInMemoryRecord(t *testing.T) {
	c, _ := recordingSignals()
	r, calls := newTestRecorder(c)
	result := &cyclestate.CycleResult{Cycle: 9}
	var timings []phasetiming.Entry
	r.Record(result, &timings, t.TempDir(), scoutOutcome())
	if len(*calls) != 1 || (*calls)[0].cycle != 9 || (*calls)[0].out.EndedAt != "2026-09-13T01:02:03Z" || (*calls)[0].out.Archetype != "role-scout" {
		t.Fatalf("emit is called once with the cycle and the stamped outcome: %+v", *calls)
	}
}

func TestRecorder_Record_EmptyWorkspaceKeepsTheMemoryRecordAndSignalsSkipped(t *testing.T) {
	c, got := recordingSignals()
	r, calls := newTestRecorder(c)
	result := &cyclestate.CycleResult{Cycle: 7}
	var timings []phasetiming.Entry
	r.Record(result, &timings, "", scoutOutcome())
	if len(result.PhasesRun) != 1 || len(timings) != 1 || len(*calls) != 1 {
		t.Fatalf("the in-memory record and the emission still happen: phases=%v timings=%d emits=%d", result.PhasesRun, len(timings), len(*calls))
	}
	if _, err := os.Stat("scout-usage.json"); err == nil {
		t.Fatal("an empty workspace must not leak a CWD-relative sidecar")
	}
	if len(*got) != 1 {
		t.Fatalf("one skipped signal, got %+v", *got)
	}
	e := (*got)[0]
	if e.Module != signalcenter.ModuleOutcome || e.Kind != signalcenter.KindOutcomeWarning || e.Severity != signalcenter.SeverityWarn ||
		e.Code != CodeSidecarSkipped || e.Cycle != 7 || e.Phase != "scout" || e.Origin != "Recorder.Record" {
		t.Errorf("the skip is an outcome.warning WARN under module outcome naming the phase: %+v", e)
	}
}

func TestRecorder_Record_SidecarWriteFailureIsAWarnSignal(t *testing.T) {
	c, got := recordingSignals()
	r, _ := newTestRecorder(c)
	notADir := filepath.Join(t.TempDir(), "ws-is-a-file")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := &cyclestate.CycleResult{Cycle: 7}
	var timings []phasetiming.Entry
	r.Record(result, &timings, notADir, scoutOutcome())
	if len(*got) != 1 || (*got)[0].Code != CodeSidecarWriteFailed || (*got)[0].Fields["path"] != UsageSidecarPath(notADir, "scout") || !strings.Contains((*got)[0].Reason, "usage sidecar") {
		t.Fatalf("a failed sidecar write is a WARN naming the path: %+v", *got)
	}
}

func TestRecorder_WritePhaseTimings_AppendMergesAndWritesAtomically(t *testing.T) {
	c, got := recordingSignals()
	r, _ := newTestRecorder(c)
	ws := t.TempDir()
	disk := []phasetiming.Entry{{Phase: "scout", Verdict: "FAIL"}}
	raw, _ := json.Marshal(disk)
	if err := os.WriteFile(phasetiming.Path(ws), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	live := []phasetiming.Entry{{Phase: "scout", Verdict: "PASS"}, {Phase: "triage", Verdict: "PASS"}}
	composed := r.WritePhaseTimings(ws, live)
	if len(composed) != 3 || composed[0].Verdict != "FAIL" || composed[2].Phase != "triage" {
		t.Fatalf("disk entries first, then the live ones, nothing deduped: %+v", composed)
	}
	back, err := os.ReadFile(phasetiming.Path(ws))
	if err != nil {
		t.Fatal(err)
	}
	want, _ := json.Marshal(composed)
	if string(back) != string(want) {
		t.Errorf("the file holds exactly the composed set (compact JSON):\n got %s\nwant %s", back, want)
	}
	if _, err := os.Stat(phasetiming.Path(ws) + ".tmp"); err == nil {
		t.Error("the temp file is renamed away")
	}
	if len(*got) != 0 {
		t.Errorf("a clean write raises no signal: %+v", *got)
	}
}

func TestRecorder_WritePhaseTimings_EmptyWorkspaceSignalsSkipped(t *testing.T) {
	c, got := recordingSignals()
	r, _ := newTestRecorder(c)
	live := []phasetiming.Entry{{Phase: "scout"}}
	if out := r.WritePhaseTimings("", live); len(out) != 1 {
		t.Fatalf("the live set is returned unchanged: %+v", out)
	}
	if len(*got) != 1 || (*got)[0].Code != CodeTimingSkipped || (*got)[0].Origin != "Recorder.WritePhaseTimings" {
		t.Fatalf("an empty workspace is a WARN, the record stays in memory: %+v", *got)
	}
}

func TestRecorder_WritePhaseTimings_RenameFailureSignals(t *testing.T) {
	c, got := recordingSignals()
	r, _ := newTestRecorder(c)
	ws := t.TempDir()
	if err := os.MkdirAll(phasetiming.Path(ws), 0o755); err != nil { // a directory where the file must land: the rename fails
		t.Fatal(err)
	}
	out := r.WritePhaseTimings(ws, []phasetiming.Entry{{Phase: "scout"}})
	if len(out) != 1 {
		t.Fatalf("the composed set is still returned: %+v", out)
	}
	if len(*got) != 1 || (*got)[0].Code != CodeTimingWriteFailed || !strings.Contains((*got)[0].Reason, "rename") {
		t.Fatalf("a failed rename is a WARN naming the step: %+v", *got)
	}
}

func TestRecorder_WritePhaseTimings_TempWriteFailureSignals(t *testing.T) {
	c, got := recordingSignals()
	r, _ := newTestRecorder(c)
	ws := filepath.Join(t.TempDir(), "missing-dir") // no workspace dir: the temp write fails
	r.WritePhaseTimings(ws, []phasetiming.Entry{{Phase: "scout"}})
	if len(*got) != 1 || (*got)[0].Code != CodeTimingWriteFailed || !strings.Contains((*got)[0].Reason, "write") {
		t.Fatalf("a failed temp write is a WARN naming the step: %+v", *got)
	}
}

func TestComposePhaseTimings_DiskFirstThenLive_GarbageFallsBackToLive(t *testing.T) {
	ws := t.TempDir()
	live := []phasetiming.Entry{{Phase: "build"}}
	if got := composePhaseTimings(ws, live); len(got) != 1 {
		t.Fatalf("no file: the live set: %+v", got)
	}
	if err := os.WriteFile(phasetiming.Path(ws), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := composePhaseTimings(ws, live); len(got) != 1 {
		t.Fatalf("garbage on disk: the live set: %+v", got)
	}
	if err := os.WriteFile(phasetiming.Path(ws), []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := composePhaseTimings(ws, live); len(got) != 1 {
		t.Fatalf("an empty log on disk: the live set: %+v", got)
	}
	raw, _ := json.Marshal([]phasetiming.Entry{{Phase: "scout"}})
	if err := os.WriteFile(phasetiming.Path(ws), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := composePhaseTimings(ws, live); len(got) != 2 || got[0].Phase != "scout" || got[1].Phase != "build" {
		t.Fatalf("disk first, then live: %+v", got)
	}
}

func TestContextFillFor_UnknownTierIsAbsent(t *testing.T) {
	if ratio, hot := contextFillFor(recovery.PhaseOutcome{ResolvedModel: "not-a-tier"}); ratio != 0 || hot {
		t.Fatalf("an unknown tier yields an absent fill: ratio=%v hot=%v", ratio, hot)
	}
}

func TestWithSignals_NilIsTheNullObject(t *testing.T) {
	opts := []Option{WithSignals(nil)}
	r := NewRecorder(fixedNow, archetypeOf, func(int, recovery.PhaseOutcome) {}, opts...)
	if r.SignalsWired() {
		t.Fatal("nil is reported unwired")
	}
	wired, _ := newTestRecorder(signalcenter.New())
	if !wired.SignalsWired() {
		t.Fatal("a Center is reported wired")
	}
	result := &cyclestate.CycleResult{Cycle: 1}
	var timings []phasetiming.Entry
	r.Record(result, &timings, "", scoutOutcome()) // the skip path with no Center must not panic
	if len(timings) != 1 {
		t.Fatalf("the record still happens through the Null Object: %+v", timings)
	}
}

func TestOutcomeCodes_AreRegisteredWithDocs(t *testing.T) {
	for _, code := range []signalcenter.Code{CodeSidecarSkipped, CodeSidecarWriteFailed, CodeTimingSkipped, CodeTimingWriteFailed} {
		if m, ok := signalcenter.IsRegistered(code); !ok || m != signalcenter.ModuleOutcome {
			t.Errorf("%s must be registered under module outcome with a doc", code)
		}
	}
}

// The sidecar's bytes are the pre-extraction rendering: indent-2 JSON in this
// field order, empty stamps omitted, tokens always present (a struct is never
// omitted by omitempty) — the dossier producer and `evolve cycle timing` read
// this file.
func TestRecorder_Record_WritesTheUsageSidecarByteIdentical(t *testing.T) {
	c, _ := recordingSignals()
	r, _ := newTestRecorder(c)
	ws := t.TempDir()
	out := scoutOutcome()
	out.Tokens = cyclestate.TokenUsage{Input: 1200, Output: 300, CacheRead: 50, CacheWrite: 7}
	result := &cyclestate.CycleResult{Cycle: 7}
	var timings []phasetiming.Entry
	r.Record(result, &timings, ws, out)
	got, err := os.ReadFile(UsageSidecarPath(ws, "scout"))
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"phase\": \"scout\",\n  \"cost_usd\": 0.5,\n  \"duration_ms\": 1200,\n  \"attempt_count\": 1,\n  \"verdict\": \"PASS\",\n  \"started_at\": \"2026-09-13T01:00:00Z\",\n  \"ended_at\": \"2026-09-13T01:02:03Z\",\n  \"archetype\": \"role-scout\",\n  \"tokens\": {\n    \"input\": 1200,\n    \"output\": 300,\n    \"cache_read\": 50,\n    \"cache_write\": 7\n  }\n}"
	if string(got) != want {
		t.Errorf("sidecar bytes differ from the declared contract:\n got %s\nwant %s", got, want)
	}
	var back UsageSidecar
	if err := json.Unmarshal(got, &back); err != nil || back.Phase != "scout" || back.Tokens.Input != 1200 || back.EndedAt != "2026-09-13T01:02:03Z" {
		t.Errorf("the declared contract type reads its own file back: %+v err=%v", back, err)
	}
}

// A canonical tier yields a real fill; the timing entry carries it.
func TestRecorder_Record_StampsTheContextFillForACanonicalTier(t *testing.T) {
	c, _ := recordingSignals()
	r, _ := newTestRecorder(c)
	out := scoutOutcome()
	out.ResolvedModel = "deep"
	out.Tokens = cyclestate.TokenUsage{Input: 20000, Output: 1000}
	result := &cyclestate.CycleResult{Cycle: 7}
	var timings []phasetiming.Entry
	r.Record(result, &timings, t.TempDir(), out)
	if len(timings) != 1 || timings[0].ContextFillRatio <= 0 || timings[0].ContextFillRatio >= 1 || timings[0].ContextWindowHot {
		t.Fatalf("a canonical tier records a fill ratio in (0,1) and not hot at this usage: %+v", timings)
	}
	if ratio, _ := contextFillFor(out); ratio != timings[0].ContextFillRatio {
		t.Errorf("the entry's fill is contextFillFor's: %v vs %v", ratio, timings[0].ContextFillRatio)
	}
}

// A non-finite float is the one way a plain struct fails to encode; the
// failure is a signal and the sidecar is not truncated to nothing.
func TestRecorder_Record_MarshalFailureIsAWarnSignalAndWritesNothing(t *testing.T) {
	c, got := recordingSignals()
	r, _ := newTestRecorder(c)
	out := scoutOutcome()
	out.CostUSD = math.NaN()
	ws := t.TempDir()
	result := &cyclestate.CycleResult{Cycle: 3}
	var timings []phasetiming.Entry
	r.Record(result, &timings, ws, out)
	if len(timings) != 1 {
		t.Fatalf("the in-memory record stands: %+v", timings)
	}
	if _, err := os.Stat(UsageSidecarPath(ws, "scout")); !os.IsNotExist(err) {
		t.Fatalf("nothing is written when the sidecar cannot be encoded: %v", err)
	}
	if len(*got) != 1 || (*got)[0].Code != CodeSidecarWriteFailed || !strings.Contains((*got)[0].Reason, "marshal") || (*got)[0].Fields["path"] == "" {
		t.Fatalf("an encode failure is a WARN naming the step and the path: %+v", *got)
	}
}

func TestRecorder_WritePhaseTimings_MarshalFailureSignalsAndLeavesTheLogUntouched(t *testing.T) {
	c, got := recordingSignals()
	r, _ := newTestRecorder(c)
	ws := t.TempDir()
	prior, _ := json.Marshal([]phasetiming.Entry{{Phase: "scout"}})
	if err := os.WriteFile(phasetiming.Path(ws), prior, 0o644); err != nil {
		t.Fatal(err)
	}
	composed := r.WritePhaseTimings(ws, []phasetiming.Entry{{Phase: "build", ContextFillRatio: math.Inf(1)}})
	if len(composed) != 2 {
		t.Fatalf("the composed set is still returned: %+v", composed)
	}
	if after, _ := os.ReadFile(phasetiming.Path(ws)); string(after) != string(prior) {
		t.Fatalf("the durable log is left untouched on an encode failure: %s", after)
	}
	if _, err := os.Stat(phasetiming.Path(ws) + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("no temp file is written when the set cannot be encoded")
	}
	if len(*got) != 1 || (*got)[0].Code != CodeTimingWriteFailed || !strings.Contains((*got)[0].Reason, "marshal") {
		t.Fatalf("an encode failure is a WARN naming the step: %+v", *got)
	}
}

// The Center is read through the accessor at every use, so a Center that
// arrives after construction (the orchestrator's WithSignalCenter option
// applied late, as tests do) receives the unit's signals.
func TestWithSignals_ReadsTheCenterLive(t *testing.T) {
	var late *signalcenter.Center
	r := NewRecorder(fixedNow, archetypeOf, func(int, recovery.PhaseOutcome) {}, WithSignals(func() *signalcenter.Center { return late }))
	if r.SignalsWired() {
		t.Fatal("no Center yet: unwired")
	}
	var got []signalcenter.Event
	late = signalcenter.New()
	late.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
	if !r.SignalsWired() {
		t.Fatal("the Center installed after construction is seen")
	}
	r.WritePhaseTimings("", nil)
	if len(got) != 1 || got[0].Code != CodeTimingSkipped {
		t.Fatalf("the late Center receives the unit's signal: %+v", got)
	}
}

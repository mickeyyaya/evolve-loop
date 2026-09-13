package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// phase_timing_composition_test.go — the dossier and the durable log must
// project the SAME record. An earlier fix let the dossier compose its own view
// ("live replaces the file"); on the resume path core starts its timing slice
// EMPTY and holds only the resumed segment, so that view silently dropped the
// pre-crash prefix sitting on disk. Meanwhile the abort path flushes BEFORE the
// dossier is built and the normal path after — so a naive append would
// double-count on one path and drop on the other. One composition rule, one
// flush, both paths.

func writeTimingLog(t *testing.T, ws string, phases ...string) {
	t.Helper()
	var entries []phasetiming.Entry
	for _, p := range phases {
		entries = append(entries, phasetiming.Entry{Phase: p, Verdict: "PASS", DurationMS: 1})
	}
	body, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, phasetiming.FileName), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func phaseNames(entries []phaseTimingEntry) string {
	var names []string
	for _, e := range entries {
		names = append(names, e.Phase)
	}
	return strings.Join(names, ",")
}

// TestFlushPhaseTimings_ResumePreservesThePreCrashPrefix: the resumed segment
// is APPENDED to what is already on disk, never substituted for it.
func TestFlushPhaseTimings_ResumePreservesThePreCrashPrefix(t *testing.T) {
	ws := t.TempDir()
	writeTimingLog(t, ws, "scout", "triage", "tdd") // pre-crash attempt
	cr := &cycleRun{cs: CycleState{WorkspacePath: ws}, phaseTimings: []phaseTimingEntry{
		{Phase: "build", Verdict: "PASS"}, {Phase: "audit", Verdict: "PASS"},
	}}
	if got := phaseNames(cr.flushPhaseTimings()); got != "scout,triage,tdd,build,audit" {
		t.Fatalf("composed = %q, want the pre-crash prefix followed by the resumed segment", got)
	}
	// The file receives exactly what the dossier projects.
	onDisk, err := phasetiming.Read(ws)
	if err != nil {
		t.Fatal(err)
	}
	if got := phaseNames(onDisk); got != "scout,triage,tdd,build,audit" {
		t.Fatalf("on-disk log = %q, must equal the composed set the dossier receives", got)
	}
}

// TestFlushPhaseTimings_IsExactlyOnce: the abort path flushes (RunCycle's
// defer) BEFORE the dossier is built (abnormalEpilogue), so a second
// composition must NOT re-append its own entries.
func TestFlushPhaseTimings_IsExactlyOnce(t *testing.T) {
	ws := t.TempDir()
	cr := &cycleRun{cs: CycleState{WorkspacePath: ws}, phaseTimings: []phaseTimingEntry{
		{Phase: "scout", Verdict: "PASS"}, {Phase: "build", Verdict: "FAIL"},
	}}
	first := phaseNames(cr.flushPhaseTimings())
	second := phaseNames(cr.flushPhaseTimings())
	if first != "scout,build" || second != first {
		t.Fatalf("flush must be exactly-once: first=%q second=%q", first, second)
	}
	onDisk, err := phasetiming.Read(ws)
	if err != nil {
		t.Fatal(err)
	}
	if got := phaseNames(onDisk); got != "scout,build" {
		t.Fatalf("on-disk log = %q — a second flush must not double-append", got)
	}
}

// TestFlushPhaseTimings_DuplicatePhasesAreReality: a phase that ran twice
// (failed attempt + repair) appears twice. The log records dispatches, and the
// composition must not "tidy" that away.
func TestFlushPhaseTimings_DuplicatePhasesAreReality(t *testing.T) {
	ws := t.TempDir()
	writeTimingLog(t, ws, "build")
	cr := &cycleRun{cs: CycleState{WorkspacePath: ws}, phaseTimings: []phaseTimingEntry{{Phase: "build", Verdict: "PASS"}}}
	if got := phaseNames(cr.flushPhaseTimings()); got != "build,build" {
		t.Fatalf("composed = %q, want both dispatches — duplicates are reality, not duplication", got)
	}
}

// Unit 01 (ADR-0103): an Orchestrator assembled as a literal — the shape this
// package's tests keep — and a cycleRun with no orchestrator at all both get
// the Null-Object recorder on first use, once, so every path still has ONE
// writer; NewOrchestrator builds the wired recorder eagerly.
func TestRecorder_LiteralOrchestratorGetsTheNullObjectRecorderOnce(t *testing.T) {
	o := &Orchestrator{now: func() time.Time { return time.Date(2026, 9, 13, 3, 4, 5, 0, time.UTC) }}
	first := o.recorder()
	if first == nil || first.SignalsWired() {
		t.Fatalf("a literal orchestrator gets an unwired recorder: %v", first)
	}
	if o.recorder() != first {
		t.Fatal("the lazily built recorder is kept, not rebuilt")
	}
	var result CycleResult
	var timings []phaseTimingEntry
	o.recordPhaseOutcome(&result, &timings, "", recovery.PhaseOutcome{Phase: "scout", Verdict: "PASS"})
	if len(timings) != 1 || timings[0].EndedAt != "2026-09-13T03:04:05Z" {
		t.Fatalf("the literal orchestrator's clock is read live through the facade: %+v", timings)
	}
	bare := (*Orchestrator)(nil).recorder()
	if got := bare.WritePhaseTimings("", timings); len(got) != 1 {
		t.Fatalf("a nil orchestrator still flushes through a recorder: %+v", got)
	}
	var bareResult CycleResult
	var bareTimings []phaseTimingEntry
	bare.Record(&bareResult, &bareTimings, "", recovery.PhaseOutcome{Phase: "audit", Verdict: "PASS"})
	if len(bareTimings) != 1 || bareTimings[0].Archetype != "" {
		t.Fatalf("the Null-Object recorder records without an archetype or an emission: %+v", bareTimings)
	}
	wired := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(signalcenter.New()))
	if !wired.recorder().SignalsWired() {
		t.Fatal("NewOrchestrator wires the recorder to the root's Center")
	}
}

// Unit 01 (ADR-0103), architecture review MEDIUM-1: the recorder reads the
// orchestrator's Center through an accessor, so WithSignalCenter applied
// after NewOrchestrator (the shape resume_lifecycle_test.go uses) still
// routes the recorder's own warnings — a snapshot at construction would
// have kept them silent for the orchestrator's whole life.
func TestRecorder_SeesASignalCenterAppliedAfterConstruction(t *testing.T) {
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	if o.recorder().SignalsWired() {
		t.Fatal("no Center at construction: the recorder is unwired")
	}
	signals, got := recordingCenter()
	WithSignalCenter(signals)(o)
	if !o.recorder().SignalsWired() {
		t.Fatal("the late Center is seen through the accessor, not a rebuilt recorder")
	}
	o.recorder().WritePhaseTimings("", nil)
	if warned := eventsOfKind(*got, signalcenter.KindOutcomeWarning); len(warned) != 1 || warned[0].Code != "OUTCOME_TIMING_SKIPPED" {
		t.Fatalf("the recorder's warning reaches the late Center: %+v", *got)
	}
}

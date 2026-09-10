package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
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

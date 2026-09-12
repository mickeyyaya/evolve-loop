package signalcenter

// sinks_test.go — the two default listeners and the filter (ADR-0101 decision 6;
// design §6.8–6.9, §9). The NDJSON sink is the durable record (everything, one line
// per event, one open handle per cycle); the stderr sink is the ONE module-tagged
// log line; Filter is how the root keeps INFO out of the console.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func readLines(t *testing.T, path string) []Event {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var out []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<20)
	for sc.Scan() {
		var e Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("line %d of %s is not one JSON event: %v: %s", len(out)+1, path, err, sc.Bytes())
		}
		out = append(out, e)
	}
	return out
}

func TestNDJSONSink_OneLinePerEventPerCycleFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pathFor := func(cycle int) string {
		if cycle == 0 {
			return ""
		}
		return filepath.Join(root, "cycle-"+itoa(cycle), "signals.ndjson")
	}
	c := New(WithPID(77))
	c.Subscribe(c.NDJSONSink(pathFor))
	for i := 0; i < 100; i++ {
		e := infoEvent("c1")
		e.Cycle = 1
		c.Emit(e)
	}
	e2 := infoEvent("c2")
	e2.Cycle = 2
	c.Emit(e2)
	e1 := infoEvent("back to c1")
	e1.Cycle = 1
	c.Emit(e1)
	one := readLines(t, pathFor(1))
	two := readLines(t, pathFor(2))
	if len(one) != 101 || len(two) != 1 {
		t.Fatalf("cycle files: %d + %d lines, want 101 + 1 (no truncation when the handle is reopened)", len(one), len(two))
	}
	for i := 1; i < len(one); i++ {
		if one[i].Seq <= one[i-1].Seq {
			t.Fatalf("seq must be monotonic in the file: %d after %d", one[i].Seq, one[i-1].Seq)
		}
	}
	if one[100].Reason != "back to c1" || one[100].PID != 77 || one[100].SchemaVersion != SchemaVersion {
		t.Errorf("the durable line carries pid + schema: %+v", one[100])
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestNDJSONSink_ResolverEmptyKeepsCountAndReportsOnce(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	c := New()
	c.Subscribe(c.NDJSONSink(func(cycle int) string {
		if cycle == 0 {
			return ""
		}
		return filepath.Join(root, "signals.ndjson")
	}))
	c.Emit(infoEvent("process-level 1")) // cycle 0 → no file yet
	c.Emit(infoEvent("process-level 2"))
	if _, err := os.Stat(filepath.Join(root, "signals.ndjson")); !os.IsNotExist(err) {
		t.Fatal("no file may be written while the resolver returns \"\"")
	}
	e := infoEvent("first cycle event")
	e.Cycle = 3
	c.Emit(e)
	lines := readLines(t, filepath.Join(root, "signals.ndjson"))
	if len(lines) != 2 {
		t.Fatalf("the cycle event and ONE drop report, got %d: %+v", len(lines), lines)
	}
	if lines[0].Reason != "first cycle event" {
		t.Errorf("the triggering event is written first: %+v", lines[0])
	}
	rep := lines[1]
	if rep.Kind != KindSinkDropped || rep.Code != CodeSinkDropped || rep.Severity != SeverityWarn || rep.Fields["dropped"] != "2" || rep.Module != ModuleSignalCenter {
		t.Errorf("the report is signalcenter.sink_dropped WARN with the count: %+v", rep)
	}
	if recent := c.Recent(); len(recent) != 4 {
		t.Errorf("dropped-from-file events are still in Recent (%d), the report too", len(recent))
	}
	e.Reason = "second cycle event"
	c.Emit(e)
	if lines := readLines(t, filepath.Join(root, "signals.ndjson")); len(lines) != 3 {
		t.Errorf("the report is emitted once, not on every later write: %d lines", len(lines))
	}
}

func TestStderrSink_GoldenLines(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	c := New(WithClock(fixedClock()), WithPID(1))
	c.Subscribe(StderrSink(&buf))
	full := Event{
		Cycle: 1636, RunID: "01J", Phase: "triage", Attempt: 1, Module: ModuleOrchestrator,
		Origin: "Orchestrator.recordPhaseOutcome", Kind: KindPhaseOutcome, Code: "ORCHESTRATOR_PHASE_VERDICT_FAIL",
		Severity: SeverityWarn, Reason: "triage verdict=FAIL: top_n card names protected surface",
		Fields: map[string]string{"verdict": "FAIL", "archetype": "plan"},
	}
	RegisterCode(ModuleOrchestrator, "ORCHESTRATOR_PHASE_VERDICT_FAIL", "test fixture: a phase recorded verdict FAIL")
	c.Emit(full)
	c.Emit(infoEvent("wave 1 open"))
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("one line per event, got %d: %q", len(lines), buf.String())
	}
	wantFull := `[orchestrator] phase.outcome WARN ORCHESTRATOR_PHASE_VERDICT_FAIL cycle=1636 phase=triage attempt=1 seq=1 origin=Orchestrator.recordPhaseOutcome — triage verdict=FAIL: top_n card names protected surface archetype=plan verdict=FAIL`
	if lines[0] != wantFull {
		t.Errorf("full line drifted:\n got %s\nwant %s", lines[0], wantFull)
	}
	wantMin := `[loop] loop.wave INFO seq=2 origin=Batch.run — wave 1 open`
	if lines[1] != wantMin {
		t.Errorf("minimal line omits empty code/cycle/phase/attempt/fields:\n got %s\nwant %s", lines[1], wantMin)
	}
}

func TestStderrSink_NeverEmitsRawControlCharacters(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	c := New()
	c.Subscribe(StderrSink(&buf))
	e := infoEvent("reason with\nnewline and \x1b[31mescape")
	e.Fields = map[string]string{"k": "v\nv"}
	c.Emit(e)
	if strings.Count(buf.String(), "\n") != 1 || strings.Contains(buf.String(), "\x1b") {
		t.Errorf("exactly one line, no terminal control bytes: %q", buf.String())
	}
}

func TestFilter_PassesAtOrAboveTheThreshold(t *testing.T) {
	t.Parallel()
	c := New()
	var got []Severity
	c.Subscribe(Filter(func(e Event) { got = append(got, e.Severity) }, SeverityWarn))
	c.Emit(infoEvent("info"))
	w := validEvent()
	c.Emit(w)
	inc := validEvent()
	inc.Kind, inc.Severity, inc.Code = KindSystemFailure, SeverityIncident, "SHIP_GIT_PUSH_REJECTED"
	c.Emit(inc)
	if len(got) != 2 || got[0] != SeverityWarn || got[1] != SeverityIncident {
		t.Errorf("Filter(WARN) passes WARN and INCIDENT only, got %v", got)
	}
}

// A resumed cycle appends to the same signals.ndjson from a NEW process: fresh
// pid, fresh seq, same file (O_APPEND). Both orders stay reconstructible by
// pid+seq — the durable record never needs a lock across processes (design §6.8).
func TestNDJSONSink_TwoProcessesAppendToTheSameCycleFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "signals.ndjson")
	resolver := func(int) string { return path }
	original := New(WithPID(101))
	original.Subscribe(original.NDJSONSink(resolver))
	original.Emit(infoEvent("original process, event 1"))
	original.Emit(infoEvent("original process, event 2"))
	resumed := New(WithPID(202))
	resumed.Subscribe(resumed.NDJSONSink(resolver))
	resumed.Emit(infoEvent("resumed process, event 1"))
	original.Emit(infoEvent("original process, event 3"))

	lines := readLines(t, path)
	if len(lines) != 4 {
		t.Fatalf("both processes append to one file: got %d lines", len(lines))
	}
	lastSeq := map[int]uint64{}
	for _, e := range lines {
		if e.PID != 101 && e.PID != 202 {
			t.Errorf("every line carries the writer's pid: %+v", e)
		}
		if e.Seq <= lastSeq[e.PID] {
			t.Errorf("seq is monotonic within a pid: %+v after %d", e, lastSeq[e.PID])
		}
		lastSeq[e.PID] = e.Seq
	}
	if lastSeq[101] != 3 || lastSeq[202] != 1 {
		t.Errorf("per-process seq counts: %v", lastSeq)
	}
}

// A Listener is a plain func: it can run outside a drain (a direct call, or a
// sink subscribed to a second Center). The drop report must be emitted with
// the sink's own lock released, or the synchronous drain that follows
// re-enters write and self-deadlocks (architecture review, S1).
func TestNDJSONSink_DropReportIsEmittedOutsideTheSinkLock(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "signals.ndjson")
	c := New()
	sink := c.NDJSONSink(func(cycle int) string {
		if cycle == 0 {
			return ""
		}
		return path
	})
	c.Subscribe(sink)
	done := make(chan struct{})
	go func() {
		defer close(done)
		sink(Event{Cycle: 0, Module: ModuleLoop, Origin: "Batch.run", Kind: KindLoopWave, Severity: SeverityInfo, Reason: "dropped: no path yet"})
		sink(Event{Cycle: 3, Module: ModuleLoop, Origin: "Batch.run", Kind: KindLoopWave, Severity: SeverityInfo, Reason: "written; reports the drop"})
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the drop report deadlocked: Emit drained synchronously back into the locked sink")
	}
	lines := readLines(t, path)
	if len(lines) != 2 || lines[1].Kind != KindSinkDropped || lines[1].Fields["dropped"] != "1" {
		t.Errorf("the written line, then the drop report: %+v", lines)
	}
}

package signalcenter

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// NDJSONSink returns the durable listener: one JSON line per event appended to
// the file pathFor(cycle) names, one open handle per cycle (reopened when the
// cycle changes; never truncated). pathFor returning "" (no workspace yet —
// process-level events before a cycle is allocated) keeps the event in Recent
// only and counts it; the count is reported ONCE, as a
// signalcenter.sink_dropped WARN, on the next successful write. Open or write
// failures count the same way, so a broken sink is visible the moment it
// recovers. A nil Center yields a no-op listener.
func (c *Center) NDJSONSink(pathFor func(cycle int) string) Listener {
	if c == nil {
		return func(Event) {}
	}
	return newNDJSONSink(c, pathFor, openAppend).write
}

// openAppend is the production opener: create the directory, append-only.
func openAppend(path string) (io.WriteCloser, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
}

func newNDJSONSink(c *Center, pathFor func(int) string, open func(string) (io.WriteCloser, error)) *ndjsonSink {
	return &ndjsonSink{center: c, pathFor: pathFor, open: open}
}

type ndjsonSink struct {
	mu      sync.Mutex
	center  *Center
	pathFor func(int) string
	open    func(string) (io.WriteCloser, error)
	cycle   int
	file    io.WriteCloser
	dropped int
}

// write appends one line, then reports any drops pending before it. The
// sink's lock is released BEFORE the report is emitted: a Listener is a plain
// func that may run outside a drain (a direct call, a sink subscribed to a
// second Center), and an Emit that drains synchronously re-enters write.
func (s *ndjsonSink) write(e Event) {
	if pending := s.writeLine(e); pending > 0 {
		s.reportDrops(e, pending)
	}
}

// writeLine writes under the lock and returns the drop count to report after
// a successful write (0 when nothing was pending or this write failed too).
func (s *ndjsonSink) writeLine(e Event) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := s.pathFor(e.Cycle)
	if path == "" {
		s.dropped++
		return 0
	}
	if err := s.ensureOpen(e.Cycle, path); err != nil {
		s.dropped++
		return 0
	}
	line, _ := json.Marshal(e)
	// A partial write (disk full mid-line) is counted as a drop; the bytes
	// already on disk are not unwritten, so that cycle's file may carry one
	// broken line — readers must tolerate it (gc retention bounds the blast).
	if _, err := s.file.Write(append(line, '\n')); err != nil {
		s.dropped++
		return 0
	}
	pending := s.dropped
	s.dropped = 0
	return pending
}

func (s *ndjsonSink) ensureOpen(cycle int, path string) error {
	if s.file != nil && s.cycle == cycle {
		return nil
	}
	if s.file != nil {
		_ = s.file.Close()
		s.file = nil
	}
	f, err := s.open(path)
	if err != nil {
		return err
	}
	s.file, s.cycle = f, cycle
	return nil
}

// reportDrops emits the drop count once, with no sink lock held. Inside a
// drain the emit is queued and delivered next — which writes the report
// through this same sink, after the current line; outside a drain it is
// delivered synchronously, which is why the lock must already be released.
func (s *ndjsonSink) reportDrops(e Event, n int) {
	s.center.Emit(Event{
		Module: ModuleSignalCenter, Origin: "ndjsonSink.write", Kind: KindSinkDropped, Code: CodeSinkDropped,
		Severity: SeverityWarn, Cycle: e.Cycle, RunID: e.RunID,
		Reason: "the durable sink had no writable path for " + strconv.Itoa(n) + " event(s) before this write; they remain in Recent only",
		Fields: map[string]string{"dropped": strconv.Itoa(n)},
	})
}

// StderrSink returns the ONE module-tagged log line renderer (design §9):
//
//	[module] kind SEVERITY CODE cycle=N phase=p attempt=k seq=s origin=Type.Method — reason k=v …
//
// Empty code/cycle/phase/attempt are omitted; fields are sorted by key. Values
// were sanitized by Normalize, so the line never carries a raw control byte.
func StderrSink(w io.Writer) Listener {
	return func(e Event) { _, _ = io.WriteString(w, FormatLine(e)+"\n") }
}

// FormatLine renders one event in the stderr line format (exported so the
// format has exactly one home and one golden test).
func FormatLine(e Event) string {
	var b strings.Builder
	b.WriteString("[" + string(e.Module) + "] " + string(e.Kind) + " " + string(e.Severity))
	if e.Code != "" {
		b.WriteString(" " + string(e.Code))
	}
	if e.Cycle != 0 {
		b.WriteString(" cycle=" + strconv.Itoa(e.Cycle))
	}
	if e.Phase != "" {
		b.WriteString(" phase=" + e.Phase)
	}
	if e.Attempt != 0 {
		b.WriteString(" attempt=" + strconv.Itoa(e.Attempt))
	}
	b.WriteString(" seq=" + strconv.FormatUint(e.Seq, 10) + " origin=" + e.Origin + " — " + e.Reason)
	keys := make([]string, 0, len(e.Fields))
	for k := range e.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(" " + k + "=" + e.Fields[k])
	}
	return b.String()
}

// Filter wraps l so it only receives events at or above at — the root uses it
// to keep INFO off the console (the contract's "log only" tier).
func Filter(l Listener, at Severity) Listener {
	return func(e Event) {
		if e.Severity.AtLeast(at) {
			l(e)
		}
	}
}

// ConsoleSink is the console half of every composition root's topology: the
// stderr line renderer at WARN and above — the severity contract's "log only"
// INFO tier stays in the durable file (design §6.9, §7). The threshold has
// this ONE home; the orchestrator roots (newRootSignalCenter) and the
// `evolve phase-observer` subprocess consume it.
func ConsoleSink(w io.Writer) Listener {
	return Filter(StderrSink(w), SeverityWarn)
}

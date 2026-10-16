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

func (c *Center) NDJSONSink(pathFor func(cycle int) string) Listener {
	if c == nil {
		return func(Event) {}
	}
	return newNDJSONSink(c, pathFor, openAppend).write
}

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

func (s *ndjsonSink) write(e Event) {
	if pending := s.writeLine(e); pending > 0 {
		s.reportDrops(e, pending)
	}
}

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

func (s *ndjsonSink) reportDrops(e Event, n int) {
	s.center.Emit(Event{
		Module: ModuleSignalCenter, Origin: "ndjsonSink.write", Kind: KindSinkDropped, Code: CodeSinkDropped,
		Severity: SeverityWarn, Cycle: e.Cycle, RunID: e.RunID,
		Reason: "the durable sink had no writable path for " + strconv.Itoa(n) + " event(s) before this write; they remain in Recent only",
		Fields: map[string]string{"dropped": strconv.Itoa(n)},
	})
}

func StderrSink(w io.Writer) Listener {
	return func(e Event) { _, _ = io.WriteString(w, FormatLine(e)+"\n") }
}

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

func Filter(l Listener, at Severity) Listener {
	return func(e Event) {
		if e.Severity.AtLeast(at) {
			l(e)
		}
	}
}

func ConsoleSink(w io.Writer) Listener {
	return Filter(StderrSink(w), SeverityWarn)
}

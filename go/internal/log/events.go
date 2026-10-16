// Package log emits structured events into JSONL sidecar files and into slog channels.
// See docs/architecture/packages/internal-log.md.
package log

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Event struct {
	EventType string
	Timestamp time.Time
	Severity  string
	Fields    map[string]any
}

type SidecarWriter struct {
	path       string
	mu         sync.Mutex
	now        func() time.Time
	appendLine func(f *os.File, line []byte) error
}

func NewSidecarWriter(path string) *SidecarWriter {
	return &SidecarWriter{path: path, now: time.Now, appendLine: writeLine}
}

func writeLine(f *os.File, line []byte) error {
	_, err := f.Write(line)
	return err
}

func (w *SidecarWriter) Close() error { return nil }

func (w *SidecarWriter) EmitAbnormal(ev Event) error {
	if ev.EventType == "" {
		return errors.New("log: Event.EventType is required")
	}
	if ev.Timestamp.IsZero() {
		ev.Timestamp = w.now().UTC()
	} else {
		ev.Timestamp = ev.Timestamp.UTC()
	}

	out := map[string]any{
		"event_type": ev.EventType,
		"timestamp":  ev.Timestamp.Format(time.RFC3339),
	}
	if ev.Severity != "" {
		out["severity"] = ev.Severity
	}
	for k, v := range ev.Fields {
		switch k {
		case "event_type", "timestamp":
			continue
		}
		out[k] = v
	}

	buf, err := marshalSorted(out)
	if err != nil {
		return fmt.Errorf("log: marshal event: %w", err)
	}
	buf = append(buf, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(w.path), 0o755); err != nil {
		return fmt.Errorf("log: create sidecar parent dir: %w", err)
	}
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("log: open sidecar %s: %w", w.path, err)
	}
	defer func() { _ = f.Close() }()
	if err := w.appendLine(f, buf); err != nil {
		return fmt.Errorf("log: append sidecar %s: %w", w.path, err)
	}
	return nil
}

func marshalSorted(m map[string]any) ([]byte, error) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	type pair struct {
		K string
		V any
	}
	ordered := make([]pair, len(keys))
	for i, k := range keys {
		ordered[i] = pair{K: k, V: m[k]}
	}
	buf := []byte{'{'}
	for i, p := range ordered {
		if i > 0 {
			buf = append(buf, ',')
		}
		kb, err := json.Marshal(p.K)
		if err != nil {
			return nil, err
		}
		buf = append(buf, kb...)
		buf = append(buf, ':')
		vb, err := json.Marshal(p.V)
		if err != nil {
			return nil, err
		}
		buf = append(buf, vb...)
	}
	buf = append(buf, '}')
	return buf, nil
}

func EmitPhase(logger *slog.Logger, phase, event string, extra map[string]any) {
	if logger == nil {
		return
	}
	attrs := []any{slog.String("phase", phase), slog.String("event", event)}
	for k, v := range extra {
		attrs = append(attrs, slog.Any(k, v))
	}
	logger.LogAttrs(context.TODO(), slog.LevelInfo, "phase", argsToAttrs(attrs)...)
}

func argsToAttrs(args []any) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(args))
	for _, a := range args {
		if at, ok := a.(slog.Attr); ok {
			attrs = append(attrs, at)
		}
	}
	return attrs
}

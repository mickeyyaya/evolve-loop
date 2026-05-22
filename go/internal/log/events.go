// Package log emits structured events into JSONL sidecar files and into
// slog channels.
//
// Two distinct surfaces:
//   - EmitAbnormal writes one JSON line into an abnormal-events.jsonl sidecar
//     (matches the .evolve/abnormal-events.jsonl shape used by bash hooks).
//   - EmitPhase records a phase lifecycle event into a slog.Logger.
//
// Both are safe for concurrent use.
package log

import (
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

// Event is the payload accepted by EmitAbnormal. EventType is required.
// Timestamp defaults to time.Now().UTC() when zero. Severity is recorded
// as a top-level "severity" field when non-empty. Fields are flattened
// into the output JSON alongside event_type/timestamp/severity.
type Event struct {
	EventType string
	Timestamp time.Time
	Severity  string
	Fields    map[string]any
}

// SidecarWriter appends Events as JSONL into a file path. The parent
// directory is created lazily on first emit. The writer is safe under
// concurrent use; a mutex serialises file writes so each line is atomic.
type SidecarWriter struct {
	path string
	mu   sync.Mutex
	now  func() time.Time // injection seam for tests; defaults to time.Now
}

// NewSidecarWriter returns a writer that will append to path. The file is
// opened lazily; if path's parent directory does not exist it is created
// on first emit.
func NewSidecarWriter(path string) *SidecarWriter {
	return &SidecarWriter{path: path, now: time.Now}
}

// Close is a no-op today (the writer opens+closes the file per emit so
// long-running processes don't leak file descriptors and rotation by
// external tools keeps working). Future O_APPEND fd reuse can land
// without changing the contract.
func (w *SidecarWriter) Close() error { return nil }

// EmitAbnormal appends one JSON line to the configured sidecar path.
// Returns an error only on EventType validation failure or I/O failure;
// concurrent emits never tear (mutex-guarded write of a single newline-
// terminated buffer).
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
		case "event_type", "timestamp": // never let Fields override the canonical keys
			continue
		}
		out[k] = v
	}

	// json.Marshal with a sorted-key envelope keeps lines diff-friendly.
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
	defer f.Close()
	if _, err := f.Write(buf); err != nil {
		return fmt.Errorf("log: append sidecar %s: %w", w.path, err)
	}
	return nil
}

// marshalSorted serialises a map with keys in lexical order. Plain
// json.Marshal on map[string]any already sorts keys (Go 1.12+), but we
// route through this helper so future custom encoders (e.g. compact
// numeric formatting) stay swappable.
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
	// Encode via a small custom path so we control key order even if
	// json.Marshal's map sort behaviour ever changes.
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

// EmitPhase records a phase lifecycle event into a slog.Logger. The
// shape is `{"phase":"…","event":"…", ...extra}` plus slog's own time
// and level fields. extra keys collide-safe with phase/event (extra wins
// is a deliberate caller-trust call — callers controlling both sides).
func EmitPhase(logger *slog.Logger, phase, event string, extra map[string]any) {
	if logger == nil {
		return
	}
	attrs := []any{slog.String("phase", phase), slog.String("event", event)}
	for k, v := range extra {
		attrs = append(attrs, slog.Any(k, v))
	}
	logger.LogAttrs(nil, slog.LevelInfo, "phase", argsToAttrs(attrs)...)
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

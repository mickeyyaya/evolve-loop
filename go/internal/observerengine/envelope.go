package observerengine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// tsLayout is the envelope and report timestamp layout (schema 1.0).
const tsLayout = "2006-01-02T15:04:05Z"

// The ops an event-append fault names.
const (
	opMarshal = "marshal"
	opMkdir   = "mkdir"
	opWrite   = "write"
)

// envelope builds one schema-1.0 event (a map, so json.Marshal keeps the
// sorted key order the goldens pin); one clock read per envelope.
func (e *Engine) envelope(eventType, severity string, data map[string]any) map[string]any {
	now := e.d.Now()
	return map[string]any{
		"id":             fmt.Sprintf("obs_%d_%d_%d", now.UnixNano(), e.s.PID, e.eventCount),
		"schema_version": "1.0",
		"ts":             now.UTC().Format(tsLayout),
		"trace_id":       e.traceID,
		"source": map[string]any{
			"component":    "phase-observer",
			"cycle":        e.s.Cycle,
			"phase":        e.s.Phase,
			"agent":        e.s.Agent,
			"observer_pid": e.s.PID,
		},
		"type":     eventType,
		"severity": severity,
		"data":     data,
	}
}

// emit appends one envelope to the events file, reports a lost line once per
// op, and retains an INCIDENT for the report whenever the open succeeded —
// regardless of the write result (the original retention point).
func (e *Engine) emit(origin, eventType, severity string, data map[string]any) {
	env := e.envelope(eventType, severity, data)
	op, err := e.appendLine(env)
	if err != nil {
		e.faultOnce(origin, CodeEventAppendFailed, op, err.Error(), map[string]string{
			"step": "emit", "op": op, "path": e.s.Paths.Events, "event_type": eventType,
		})
	}
	if severity == "INCIDENT" && (err == nil || op == opWrite) {
		e.incidents = append(e.incidents, env)
	}
}

// appendLine marshals, creates the directory, opens append-only and writes
// one line; on failure it names the op that failed.
func (e *Engine) appendLine(env map[string]any) (op string, err error) {
	b, err := json.Marshal(env)
	if err != nil {
		return opMarshal, err
	}
	if err := os.MkdirAll(filepath.Dir(e.s.Paths.Events), 0o755); err != nil {
		return opMkdir, err
	}
	f, err := e.openAppend(e.s.Paths.Events)
	if err != nil {
		return opOpen, err
	}
	defer func() { _ = f.Close() }() // best-effort append; the write result is already reported
	if _, err := f.Write(append(b, '\n')); err != nil {
		return opWrite, err
	}
	return "", nil
}

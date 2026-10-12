package observerengine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const tsLayout = "2006-01-02T15:04:05Z"

const (
	opMarshal = "marshal"
	opMkdir   = "mkdir"
	opWrite   = "write"
)

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
	defer func() { _ = f.Close() }()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return opWrite, err
	}
	return "", nil
}

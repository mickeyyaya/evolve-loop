package observerengine

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// report is the pure phase summary — the 18 keys of schema 1.0; one clock
// read.
func (e *Engine) report() map[string]any {
	now := e.d.Now()
	return map[string]any{
		"schema_version":        "1.0",
		"trace_id":              e.traceID,
		"started_at":            e.startedAtISO,
		"finished_at":           now.UTC().Format(tsLayout),
		"duration_s":            int(now.Sub(e.startedAt).Seconds()),
		"cycle":                 e.s.Cycle,
		"phase":                 e.s.Phase,
		"agent":                 e.s.Agent,
		"event_count":           e.eventCount,
		"tool_call_count":       e.toolCallCount,
		"tool_result_count":     e.toolResultCnt,
		"error_count":           e.errorCount,
		"rate_limit_count":      e.rateLimitCnt,
		"cumulative_cost":       e.cumulativeCost,
		"cache_read_tokens":     e.cacheReadTok,
		"cache_creation_tokens": e.cacheCreateTok,
		"incident_count":        len(e.incidents),
		"incidents":             e.incidents,
	}
}

// WriteReport persists the summary atomically (.tmp + rename). A failure is
// reported as OBSERVER_REPORT_WRITE_FAILED AND returned; the host keeps its
// exit code (the report is best-effort). The marshal and the write fold into
// one error chain — the marshal of numbers, strings and already-marshalled
// maps cannot fail, so the chain keeps the handling with one reachable branch.
func (e *Engine) WriteReport() error {
	b, err := json.MarshalIndent(e.report(), "", "  ")
	if err == nil {
		err = writeAtomic(e.s.Paths.Report, append(b, '\n'))
	}
	if err != nil {
		e.reportFault(originWriteReport, CodeReportWriteFailed, err.Error(), map[string]string{"step": "report", "path": e.s.Paths.Report})
	}
	return err
}

// writeAtomic creates the directory, writes <path>.tmp and renames it over
// the target.
func writeAtomic(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

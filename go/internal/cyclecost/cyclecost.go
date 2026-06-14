// Package cyclecost computes per-phase + per-cycle cost from the unified
// phasestream event stream (ADR-0020). Walks <workspace>/*-events.ndjson
// files, extracts the last kind==result envelope from each, and sums
// cost + token counts into a per-phase + per-cycle summary.
//
// Used by cmd_loop after each cycle to accumulate the batch cost total
// for display-only telemetry (`total_cost_usd` in loop output). Cost no
// longer gates anything — the token-budget cost gates were removed.
//
// Source format: each line is one normalized envelope. The result
// envelope carries data.cost_usd plus data.tokens.{in,out,cache_r,
// cache_c} (the shape phasestream.Classifier emits). Raw-log parsing
// quirks (legacy single-blob JSON, mid-stream malformed events) are
// handled upstream by the normalizer, so there is no raw fallback here.
//
// The emitted Summary/PhaseCost JSON shape is frozen — it mirrors what
// show-cycle-cost.sh --json produced, so downstream tooling that grep'd
// that output keeps working byte-for-byte.
//
// Zero-cost paths are not errors. A missing workspace IS an error
// — caller already failed to enter the cycle, summing zero would
// mask the bug.
package cyclecost

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PhaseCost captures one phase's contribution. Field names mirror the
// JSON shape show-cycle-cost.sh --json emits, so downstream tooling
// that already grep'd the bash output keeps working byte-for-byte.
type PhaseCost struct {
	Phase                    string  `json:"phase,omitempty"`
	CostUSD                  float64 `json:"cost_usd"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
	InputTokens              int64   `json:"input_tokens"`
}

// Summary is the per-cycle aggregate. Fits the bash --json shape so a
// future Go-callable wrapper can swap show-cycle-cost.sh without
// downstream changes.
type Summary struct {
	Cycle  int         `json:"cycle"`
	Phases []PhaseCost `json:"phases"`
	Total  PhaseCost   `json:"total"`
}

// ErrNoLogs is returned when the workspace exists but has no
// *-events.ndjson files. Distinguishes from ErrNoWorkspace (cycle never
// reached any phase) — both are cost=$0 but the cause is different.
var ErrNoLogs = errors.New("cyclecost: no *-events.ndjson files in workspace")

// ErrNoWorkspace is returned when the workspace dir doesn't exist.
var ErrNoWorkspace = errors.New("cyclecost: workspace does not exist")

// globFn is a test seam — production calls filepath.Glob directly,
// but with literal patterns Glob cannot fail in practice. Tests swap
// this to drive the defensive error path.
var globFn = filepath.Glob

// maxScannerBufBytes controls the per-line read buffer cap. Production
// uses 16MB to accommodate huge stream-json events. Tests can lower
// this to drive the scanner.Err() (bufio.ErrTooLong) branch without
// writing a 17MB fixture.
var maxScannerBufBytes = 1 << 24 // 16MB

// SummarizeCycle walks the workspace, sums per-phase costs, returns
// a Summary. Cycle is just metadata — used only for the returned
// Summary.Cycle field.
//
// Returns ErrNoLogs / ErrNoWorkspace for empty / missing inputs so
// callers can distinguish "cycle ran but produced no telemetry"
// (e.g., orchestrator crashed before any phase) from "cycle ran
// fine and just cost nothing" (unlikely but possible with a fully-
// cached repeat invocation).
func SummarizeCycle(workspace string, cycle int) (Summary, error) {
	info, err := os.Stat(workspace)
	if err != nil {
		if os.IsNotExist(err) {
			return Summary{Cycle: cycle}, ErrNoWorkspace
		}
		return Summary{Cycle: cycle}, fmt.Errorf("stat workspace: %w", err)
	}
	if !info.IsDir() {
		return Summary{Cycle: cycle}, ErrNoWorkspace
	}

	logs, err := globFn(filepath.Join(workspace, "*-events.ndjson"))
	if err != nil {
		return Summary{Cycle: cycle}, fmt.Errorf("glob: %w", err)
	}
	if len(logs) == 0 {
		return Summary{Cycle: cycle}, ErrNoLogs
	}
	sort.Strings(logs) // deterministic phase order across runs

	summary := Summary{Cycle: cycle}
	for _, log := range logs {
		pc, ok := parseEventsLog(log)
		if !ok {
			continue
		}
		summary.Phases = append(summary.Phases, pc)
		summary.Total.CostUSD += pc.CostUSD
		summary.Total.CacheReadInputTokens += pc.CacheReadInputTokens
		summary.Total.CacheCreationInputTokens += pc.CacheCreationInputTokens
		summary.Total.OutputTokens += pc.OutputTokens
		summary.Total.InputTokens += pc.InputTokens
	}
	return summary, nil
}

// eventEnvelope matches the subset of a phasestream envelope cyclecost
// needs: the kind discriminator plus the result event's cost + token
// payload (data.cost_usd, data.tokens.{in,out,cache_r,cache_c}).
//
// Decoding into this typed struct (not map[string]any) is deliberate:
// json.Unmarshal reads the token counts straight into int64, avoiding the
// float64 default a generic map decode imposes on JSON numbers.
type eventEnvelope struct {
	Kind string `json:"kind"`
	Data struct {
		CostUSD float64 `json:"cost_usd"`
		Tokens  struct {
			In     int64 `json:"in"`
			Out    int64 `json:"out"`
			CacheR int64 `json:"cache_r"`
			CacheC int64 `json:"cache_c"`
		} `json:"tokens"`
	} `json:"data"`
}

// parseEventsLog reads one *-events.ndjson, picks the last kind==result
// envelope, and returns the parsed phase cost. Returns ok=false when no
// usable result envelope is found (the phase contributes nothing — there
// is no raw fallback, since the normalizer already produced clean events).
//
// Phase name is derived by stripping `-events.ndjson` from the basename:
//
//	`scout-events.ndjson` → `scout`
//	`subagent.scout.parallel-worker-1-events.ndjson` → `subagent.scout.parallel-worker-1`
func parseEventsLog(logPath string) (PhaseCost, bool) {
	f, err := os.Open(logPath)
	if err != nil {
		return PhaseCost{}, false
	}
	defer func() { _ = f.Close() }()

	phase := strings.TrimSuffix(filepath.Base(logPath), "-events.ndjson")

	// Walk every line; remember the LAST one that decodes as a result
	// envelope.
	scanner := bufio.NewScanner(f)
	// Allow long lines — a result envelope can embed large payloads.
	scanner.Buffer(make([]byte, 1<<10), maxScannerBufBytes)

	var last eventEnvelope
	var found bool
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Cheap pre-check: skip the JSON parse for the many non-result
		// envelopes per phase. A false positive (the substring appearing
		// inside a data payload) is harmless — the ev.Kind == "result"
		// check below re-validates after the parse.
		if !strings.Contains(line, `"kind":"result"`) {
			continue
		}
		var ev eventEnvelope
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if ev.Kind == "result" {
			last = ev
			found = true
		}
	}
	if err := scanner.Err(); err != nil {
		return PhaseCost{}, false
	}
	if !found {
		return PhaseCost{}, false
	}

	return PhaseCost{
		Phase:                    phase,
		CostUSD:                  last.Data.CostUSD,
		CacheReadInputTokens:     last.Data.Tokens.CacheR,
		CacheCreationInputTokens: last.Data.Tokens.CacheC,
		OutputTokens:             last.Data.Tokens.Out,
		InputTokens:              last.Data.Tokens.In,
	}, true
}

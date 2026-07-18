// Package dispatchevents writes abnormal-events.jsonl entries from the
// dispatcher level — counter-non-advance, circuit-breaker trips, verify
// failures, classifier verdicts. Port of the inline JSONL emission at
// archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh:983-993 plus the
// post-classifier records added in cycle-61 follow-ups.
//
// Distinct from internal/adapters/observer/, which watches per-phase
// stdout for stalls. dispatchevents emits at the *cycle* boundary;
// observer emits within a phase. Both write JSONL, but to different
// files and consumed by different audiences.
package dispatchevents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Severity values mirror the bash strings ("INFO" / "WARN" / "ERROR").
// Kept as a typed alias rather than a separate enum so JSON marshaling
// is a no-op.
type Severity string

const (
	SeverityInfo  Severity = "INFO"
	SeverityWarn  Severity = "WARN"
	SeverityError Severity = "ERROR"
)

// EventType enumerates the dispatch-level abnormal events that
// cmd_loop emits. The string values are wire-compatible with the bash
// dispatcher.
type EventType string

const (
	EventCounterNonAdvance     EventType = "counter-non-advance"
	EventCircuitBreakerTripped EventType = "circuit-breaker-tripped"
	EventVerifyFailed          EventType = "verify-failed"
	EventClassification        EventType = "classification"
	EventGoalStallEscalated    EventType = "goal-stall-escalated"
)

// Event is one line in abnormal-events.jsonl.
//
// Fields use the bash dispatcher names verbatim so existing operator
// `jq` filters keep working. SourcePhase identifies the dispatch
// sub-system (always "dispatch" for this writer); RemediationHint is a
// short operator-readable suggestion.
type Event struct {
	EventType       EventType `json:"event_type"`
	Timestamp       string    `json:"timestamp"`
	SourcePhase     string    `json:"source_phase"`
	Severity        Severity  `json:"severity"`
	Details         string    `json:"details"`
	RemediationHint string    `json:"remediation_hint,omitempty"`
	// Cycle is the cycle number the event applies to. Useful in
	// post-hoc grep beyond what the workspace path already encodes.
	Cycle int `json:"cycle,omitempty"`
	// Classification is set on EventClassification events; mirrors the
	// cycleclassify.Classification string.
	Classification string `json:"classification,omitempty"`
}

// Writer appends to abnormal-events.jsonl in a cycle workspace.
// Safe for concurrent use across goroutines (mutex-guarded). One
// Writer per cycle workspace is the intended pattern; reuse is fine.
type Writer struct {
	path string
	mu   sync.Mutex
	now  func() time.Time
}

// NewWriter constructs a Writer that targets
// <workspace>/abnormal-events.jsonl. The workspace dir is NOT created
// automatically — the dispatcher already created it before any phase
// ran, so a failed os.MkdirAll here would mask a real bug.
func NewWriter(workspace string) *Writer {
	return &Writer{
		path: filepath.Join(workspace, "abnormal-events.jsonl"),
		now:  time.Now,
	}
}

// marshalFn + writeFn are test seams for the marshal-error and
// write-error branches. Production paths use json.Marshal +
// (*os.File).Write directly; tests replace these to drive the rare
// error paths (disk full, malformed Event) without contriving them on
// a real filesystem.
var (
	marshalFn = json.Marshal
	writeFn   = func(f *os.File, b []byte) (int, error) { return f.Write(b) }
)

// Emit serializes e and appends it to abnormal-events.jsonl. Timestamp
// is filled in automatically if zero. Returns an error only on
// marshal/I/O failures; bash equivalent uses `|| true` to swallow these
// silently, but Go callers should at least log on error.
func (w *Writer) Emit(e Event) error {
	if e.Timestamp == "" {
		e.Timestamp = w.now().UTC().Format(time.RFC3339)
	}
	if e.SourcePhase == "" {
		e.SourcePhase = "dispatch"
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	line, err := marshalFn(e)
	if err != nil {
		return fmt.Errorf("dispatchevents marshal: %w", err)
	}
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("dispatchevents open: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := writeFn(f, append(line, '\n')); err != nil {
		return fmt.Errorf("dispatchevents write: %w", err)
	}
	return nil
}

// EmitCounterNonAdvance is the shorthand for the most common dispatch
// abnormal — lastCycleNumber didn't move forward after a cycle. Used
// by cmd_loop when state.lastCycleNumber is unchanged across a
// RunCycle invocation.
func (w *Writer) EmitCounterNonAdvance(cycle int) error {
	return w.Emit(Event{
		EventType:       EventCounterNonAdvance,
		Severity:        SeverityWarn,
		Cycle:           cycle,
		Details:         fmt.Sprintf("lastCycleNumber did not advance after cycle %d — audit verdict likely WARN or FAIL", cycle),
		RemediationHint: "Check orchestrator-report.md verdict; if FAIL run retrospective; if ship.sh failed inspect ship-gate logs",
	})
}

// EmitVerifyFailed records a ledger pipeline verification failure.
// `missing` lists the roles whose ledger entries were absent.
func (w *Writer) EmitVerifyFailed(cycle int, missing []string) error {
	return w.Emit(Event{
		EventType:       EventVerifyFailed,
		Severity:        SeverityError,
		Cycle:           cycle,
		Details:         fmt.Sprintf("cycle %d pipeline incomplete: missing %v", cycle, missing),
		RemediationHint: "Inspect orchestrator-report.md for the phase that aborted; check per-role stdout/stderr logs",
	})
}

// EmitClassification records the classifier verdict. Severity is INFO
// for recoverable classes, ERROR for integrity-breach (the kernel
// signal that demands human investigation).
func (w *Writer) EmitClassification(cycle int, classification string) error {
	sev := SeverityInfo
	if classification == "integrity-breach" {
		sev = SeverityError
	}
	return w.Emit(Event{
		EventType:      EventClassification,
		Severity:       sev,
		Cycle:          cycle,
		Classification: classification,
		Details:        fmt.Sprintf("cycle %d classified as %s", cycle, classification),
	})
}

// EmitCircuitBreakerTripped records the same-cycle circuit-breaker.
// streak is the consecutive same-cycle count that crossed threshold.
func (w *Writer) EmitCircuitBreakerTripped(cycle, streak, threshold int) error {
	return w.Emit(Event{
		EventType:       EventCircuitBreakerTripped,
		Severity:        SeverityError,
		Cycle:           cycle,
		Details:         fmt.Sprintf("same cycle number %d reported %d consecutive times (threshold=%d) — dispatcher deadlocked", cycle, streak, threshold),
		RemediationHint: "Use the explicit no-worktree operator mode or raise EVOLVE_DISPATCH_REPEAT_THRESHOLD; inspect cycle workspace orchestrator-report.md",
	})
}

// EmitGoalStallEscalated records that one goal produced `streak` consecutive
// empty/blocked (non-shipping) cycles, crossing the goal-stall threshold — the
// loop self-filed an inbox todo naming the goal instead of re-dispatching it
// blindly again. goalHash identifies the stalled goal.
func (w *Writer) EmitGoalStallEscalated(cycle, streak, threshold int, goalHash string) error {
	return w.Emit(Event{
		EventType:       EventGoalStallEscalated,
		Severity:        SeverityWarn,
		Cycle:           cycle,
		Details:         fmt.Sprintf("goal %s produced %d consecutive empty/blocked cycles (threshold=%d) — nothing shipped; escalated instead of re-dispatching", goalHash, streak, threshold),
		RemediationHint: "See the auto-filed goal-stall inbox todo: re-scope or split the goal, or address the recurring block reason before re-running it",
	})
}

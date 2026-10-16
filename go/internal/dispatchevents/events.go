// Package dispatchevents writes the dispatcher-level entries of abnormal-events.jsonl.
// See docs/architecture/packages/internal-dispatchevents.md.
package dispatchevents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Severity string

const (
	SeverityInfo  Severity = "INFO"
	SeverityWarn  Severity = "WARN"
	SeverityError Severity = "ERROR"
)

type EventType string

const (
	EventCounterNonAdvance     EventType = "counter-non-advance"
	EventCircuitBreakerTripped EventType = "circuit-breaker-tripped"
	EventVerifyFailed          EventType = "verify-failed"
	EventClassification        EventType = "classification"
	EventGoalStallEscalated    EventType = "goal-stall-escalated"
	EventStallDetected         EventType = "stall-detected"
	EventPhaseOutputsSurveyed  EventType = "phase-outputs-surveyed"
)

var knownEventTypes = map[EventType]bool{
	EventCounterNonAdvance:     true,
	EventCircuitBreakerTripped: true,
	EventVerifyFailed:          true,
	EventClassification:        true,
	EventGoalStallEscalated:    true,
	EventStallDetected:         true,
	EventPhaseOutputsSurveyed:  true,
}

func IsKnownEventType(t EventType) bool { return knownEventTypes[t] }

type Event struct {
	EventType       EventType `json:"event_type"`
	Timestamp       string    `json:"timestamp"`
	SourcePhase     string    `json:"source_phase"`
	Severity        Severity  `json:"severity"`
	Details         string    `json:"details"`
	RemediationHint string    `json:"remediation_hint,omitempty"`
	Cycle           int       `json:"cycle,omitempty"`
	Classification  string    `json:"classification,omitempty"`
}

type Writer struct {
	path string
	mu   sync.Mutex
	now  func() time.Time
}

func NewWriter(workspace string) *Writer {
	return &Writer{
		path: filepath.Join(workspace, "abnormal-events.jsonl"),
		now:  time.Now,
	}
}

var (
	marshalFn = json.Marshal
	writeFn   = func(f *os.File, b []byte) (int, error) { return f.Write(b) }
)

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

func (w *Writer) EmitCounterNonAdvance(cycle int) error {
	return w.Emit(Event{
		EventType:       EventCounterNonAdvance,
		Severity:        SeverityWarn,
		Cycle:           cycle,
		Details:         fmt.Sprintf("lastCycleNumber did not advance after cycle %d — audit verdict likely WARN or FAIL", cycle),
		RemediationHint: "Check orchestrator-report.md verdict; if FAIL run retrospective; if ship.sh failed inspect ship-gate logs",
	})
}

func (w *Writer) EmitVerifyFailed(cycle int, missing []string) error {
	return w.Emit(Event{
		EventType:       EventVerifyFailed,
		Severity:        SeverityError,
		Cycle:           cycle,
		Details:         fmt.Sprintf("cycle %d pipeline incomplete: missing %v", cycle, missing),
		RemediationHint: "Inspect orchestrator-report.md for the phase that aborted; check per-role stdout/stderr logs",
	})
}

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

func (w *Writer) EmitCircuitBreakerTripped(cycle, streak, threshold int) error {
	return w.Emit(Event{
		EventType:       EventCircuitBreakerTripped,
		Severity:        SeverityError,
		Cycle:           cycle,
		Details:         fmt.Sprintf("same cycle number %d reported %d consecutive times (threshold=%d) — dispatcher deadlocked", cycle, streak, threshold),
		RemediationHint: "Use the explicit no-worktree operator mode or raise EVOLVE_DISPATCH_REPEAT_THRESHOLD; inspect cycle workspace orchestrator-report.md",
	})
}

func (w *Writer) EmitGoalStallEscalated(cycle, streak, threshold int, goalHash, outcomes string) error {
	return w.Emit(Event{
		EventType:       EventGoalStallEscalated,
		Severity:        SeverityWarn,
		Cycle:           cycle,
		Details:         fmt.Sprintf("goal %s produced %d consecutive %s cycles (threshold=%d) — nothing shipped; escalated instead of re-dispatching", goalHash, streak, outcomes, threshold),
		RemediationHint: "See the auto-filed stall inbox todo: re-scope or split the goal, or address the recurring block/fail reason before re-running it",
	})
}

func (w *Writer) EmitPhaseOutputsSurveyed(cycle int, details string, abnormal bool) error {
	e := Event{
		EventType:   EventPhaseOutputsSurveyed,
		SourcePhase: "phase-outputs",
		Severity:    SeverityInfo,
		Cycle:       cycle,
		Details:     details,
	}
	if abnormal {
		e.Severity = SeverityWarn
		e.RemediationHint = fmt.Sprintf("run `evolve cycle outputs %d` for the per-phase rows; a missing/empty artifact means that phase's work cannot be reviewed", cycle)
	}
	return w.Emit(e)
}

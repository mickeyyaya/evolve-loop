package lifecycle

// ledger.go — the chained inbox-lifecycle ledger line (inboxmover.go:104-124,
// :911-964 on the base).

import (
	"context"
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
)

// ledgerEntry is one lifecycle transition, recorded as a CHAINED ledger entry
// via ledgerLine (from/to/reason fold into the message field).
type ledgerEntry struct {
	Action string
	TaskID string
	From   string
	To     string
	Cycle  *int    // nil when no active cycle
	GitSHA *string // nil when unknown
	Reason string
}

// foldLifecycleMessage renders "from → to: reason", dropping the arrow
// segment when no paths are involved (release/recover shapes set only Reason).
func foldLifecycleMessage(from, to, reason string) string {
	if from == "" && to == "" {
		return reason
	}
	return from + " → " + to + ": " + reason
}

// ledgerLine records one inbox-lifecycle event through the CHAINED append
// path (the old raw O_APPEND write was the per-cycle chain-break generator
// under fleet concurrency). Best-effort — a failed telemetry append must not
// un-move an item that already moved — but loud, never silent: the WARN line
// is kept verbatim because the appender is an interface (fakes and other
// adapters emit nothing) and the file ledger reports its own
// LEDGER_APPEND_FAILED. A nil appender is the Null Object: nothing to append
// into, nothing said.
func (m *Mover) ledgerLine(e ledgerEntry) {
	if m.ledger == nil {
		return
	}
	cycle := 0
	if e.Cycle != nil {
		cycle = *e.Cycle
	}
	gitHead := ""
	if e.GitSHA != nil {
		gitHead = *e.GitSHA
	}
	err := m.ledger.AppendLifecycle(context.Background(), ledger.LifecycleRecord{
		TS:      m.now().UTC().Format(time.RFC3339),
		Action:  e.Action,
		TaskID:  e.TaskID,
		Cycle:   cycle,
		GitHead: gitHead,
		Message: foldLifecycleMessage(e.From, e.To, e.Reason),
	})
	if err != nil {
		m.linef("WARN: ledger append (inbox-lifecycle %s %s): %v", e.Action, e.TaskID, err)
	}
}

// intPtr returns a *int from a numeric string, or nil if empty/unparseable.
// Mirrors bash semantics: empty cycle → null; numeric → numeric (Sscanf
// accepts leading digits — a preserved leniency).
func intPtr(s string) *int {
	if s == "" {
		return nil
	}
	var v int
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
		return nil
	}
	return &v
}

// cycleOf projects a cycle string onto an event's Cycle: intPtr's value, or 0.
func cycleOf(s string) int {
	if p := intPtr(s); p != nil {
		return *p
	}
	return 0
}

// strPtr returns a *string, or nil if empty.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

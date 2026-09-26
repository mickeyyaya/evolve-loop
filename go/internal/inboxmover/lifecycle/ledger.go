package lifecycle

import (
	"context"
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
)

type ledgerEntry struct {
	Action string
	TaskID string
	From   string
	To     string
	Cycle  *int    // nil when no active cycle
	GitSHA *string // nil when unknown
	Reason string
}

func foldLifecycleMessage(from, to, reason string) string {
	if from == "" && to == "" {
		return reason
	}
	return from + " → " + to + ": " + reason
}

// ledgerLine appends e best-effort: a failed append never undoes the move, and it prints
// a WARN line because an appender need not report its own failure.
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

// intPtr parses s's leading digits (Sscanf, so "12x" is 12); empty or non-numeric is nil.
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

package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
)

type ledgerEntry struct {
	Action string
	TaskID string
	From   string
	To     string
	Cycle  *int
	GitSHA *string
	Reason string
}

func foldLifecycleMessage(from, to, reason string) string {
	if from == "" && to == "" {
		return reason
	}
	return from + " → " + to + ": " + reason
}

func (m *Mover) ledgerLine(e ledgerEntry) {
	if m.ledger == nil {
		return
	}
	if err := m.appendRecord(e); err != nil {
		m.linef("WARN: ledger append (inbox-lifecycle %s %s): %v", e.Action, e.TaskID, err)
	}
}

func (m *Mover) appendRecord(e ledgerEntry) error {
	if m.ledger == nil {
		return errors.New("no ledger is wired to record it")
	}
	cycle := 0
	if e.Cycle != nil {
		cycle = *e.Cycle
	}
	gitHead := ""
	if e.GitSHA != nil {
		gitHead = *e.GitSHA
	}
	return m.ledger.AppendLifecycle(context.Background(), ledger.LifecycleRecord{
		TS:      m.now().UTC().Format(time.RFC3339),
		Action:  e.Action,
		TaskID:  e.TaskID,
		Cycle:   cycle,
		GitHead: gitHead,
		Message: foldLifecycleMessage(e.From, e.To, e.Reason),
	})
}

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

func cycleOf(s string) int {
	if p := intPtr(s); p != nil {
		return *p
	}
	return 0
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Package cycle106 ports the cycle-106 ACS predicates to Go.
//
// Subject: build-planner Go wiring (Opt C cycle-1 shadow wire in Go
// orchestrator) + v12.1.1 release commit (bridge ExtraFlags separator +
// auto-model sentinel).
package cycle106

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// minStorage is a minimal in-memory Storage stub for orchestrator tests.
type minStorage struct{ cs core.CycleState }

func (s *minStorage) ReadState(_ context.Context) (core.State, error)  { return core.State{}, nil }
func (s *minStorage) WriteState(_ context.Context, _ core.State) error { return nil }
func (s *minStorage) ReadCycleState(_ context.Context) (core.CycleState, error) {
	return s.cs, nil
}
func (s *minStorage) WriteCycleState(_ context.Context, cs core.CycleState) error {
	s.cs = cs
	return nil
}
func (s *minStorage) AcquireLock(_ context.Context) (func() error, error) {
	return func() error { return nil }, nil
}

// minLedger is a no-op Ledger for orchestrator tests.
type minLedger struct{}

func (l *minLedger) Append(_ context.Context, _ core.LedgerEntry) error { return nil }
func (l *minLedger) Verify(_ context.Context) error                     { return nil }
func (l *minLedger) Iter(_ context.Context) (core.LedgerIterator, error) {
	return &emptyIter{}, nil
}

type emptyIter struct{}

func (e *emptyIter) Next() (core.LedgerEntry, bool, error) { return core.LedgerEntry{}, false, nil }
func (e *emptyIter) Close() error                          { return nil }

// noopRunner is a PhaseRunner that immediately returns a scripted PASS
// verdict with a given nextPhase. Used to drive the orchestrator routing
// test without invoking a real LLM bridge.
type noopRunner struct {
	phaseName string
	next      string
}

func (r *noopRunner) Name() string { return r.phaseName }
func (r *noopRunner) Run(_ context.Context, _ core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{
		Phase:     r.phaseName,
		Verdict:   core.VerdictPASS,
		NextPhase: r.next,
	}, nil
}

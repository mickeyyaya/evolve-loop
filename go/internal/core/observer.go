package core

import "context"

// Observer attaches a stall detector to each phase. Start runs before the
// phase and the orchestrator calls its returned, idempotent cancel after
// every phase, success or failure.
type Observer interface {
	// Start returns an idempotent cancel for phase; implementations must
	// not block here.
	Start(ctx context.Context, phase string, req PhaseRequest) (cancel func())
}

// noopObserver is the default when WithObserver is not set.
type noopObserver struct{}

// Start implements Observer as a no-op.
func (noopObserver) Start(_ context.Context, _ string, _ PhaseRequest) func() {
	return func() {}
}

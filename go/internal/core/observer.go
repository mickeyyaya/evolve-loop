package core

import "context"

type Observer interface {
	Start(ctx context.Context, phase string, req PhaseRequest) (cancel func())
}

type noopObserver struct{}

func (noopObserver) Start(_ context.Context, _ string, _ PhaseRequest) func() {
	return func() {}
}

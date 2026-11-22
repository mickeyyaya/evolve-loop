package inboxmover

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func mustResolveScope(t *testing.T, opts Options, cycle int, scopeIDs []string) *continuation.Continuation {
	t.Helper()
	c, err := ResolveContinuationForScope(opts, cycle, scopeIDs)
	if err != nil {
		t.Fatalf("ResolveContinuationForScope: %v", err)
	}
	return c
}

func mustReconcile(t *testing.T, opts Options) []string {
	t.Helper()
	released, err := ReconcileConsumedBindings(opts)
	if err != nil {
		t.Fatalf("ReconcileConsumedBindings: %v", err)
	}
	return released
}

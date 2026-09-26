package core_test

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

// Only this external test package can import inboxmover without an import cycle.
func init() {
	core.RealInboxForTest.Resolve = func(root string, cycle int) *continuation.Continuation {
		return inboxmover.ResolveContinuation(inboxmover.Options{ProjectRoot: root}, cycle)
	}
	core.RealInboxForTest.ResolveScope = func(root string, cycle int, scopeIDs []string) *continuation.Continuation {
		return inboxmover.ResolveContinuationForScope(inboxmover.Options{ProjectRoot: root}, cycle, scopeIDs)
	}
	core.RealInboxForTest.Release = func(root string, cycle int, reason string) error {
		_, err := inboxmover.ReleaseCycleProcessingWithReason(inboxmover.Options{ProjectRoot: root}, cycle, reason)
		return err
	}
	core.RealInboxForTest.Claim = func(root, taskID, cycle string) error {
		_, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root}, taskID, cycle)
		return err
	}
}

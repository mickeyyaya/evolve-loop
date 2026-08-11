package core_test

// continuation_adopt_inbox_test.go — injects the REAL inboxmover functions
// into package core's continuation-adopt tests (RealInboxForTest). The
// external test package is the legal home for this import: core_test →
// inboxmover → adapters/ledger → core is acyclic, while the same import from
// an in-package test file is a cycle (the chained-ledger fix made inboxmover
// depend on core transitively). init runs before any test in the shared test
// binary, so the hooks are always set.

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

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

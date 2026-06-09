package core

import (
	"errors"
	"fmt"
	"testing"
)

// Sentinel errors must round-trip through errors.Is when wrapped via %w.
// This is the contract the orchestrator and adapters rely on to make
// branching decisions ("if errors.Is(err, ErrBudgetExceeded) { … }").
func TestSentinels_ErrorsIs(t *testing.T) {
	t.Parallel()
	sentinels := []error{
		ErrPhaseGateFailed,
		ErrLedgerChainBroken,
		ErrBudgetExceeded,
		ErrLockHeld,
		ErrSubprocessNonZero,
		ErrPhaseInvalid,
		ErrTransitionInvalid,
	}
	for _, s := range sentinels {
		t.Run(s.Error(), func(t *testing.T) {
			wrapped := fmt.Errorf("layer: %w", s)
			if !errors.Is(wrapped, s) {
				t.Errorf("errors.Is(wrap(%v), %v) = false; want true", s, s)
			}
			// And distinct sentinels must NOT be aliases.
			for _, other := range sentinels {
				if other == s {
					continue
				}
				if errors.Is(s, other) {
					t.Errorf("sentinels %v and %v collide", s, other)
				}
			}
		})
	}
}

func TestSentinels_MessagesArePresent(t *testing.T) {
	t.Parallel()
	sentinels := []error{
		ErrPhaseGateFailed, ErrLedgerChainBroken, ErrBudgetExceeded,
		ErrLockHeld, ErrSubprocessNonZero, ErrPhaseInvalid, ErrTransitionInvalid,
	}
	for _, s := range sentinels {
		if s.Error() == "" {
			t.Errorf("sentinel has empty message: %v", s)
		}
	}
}

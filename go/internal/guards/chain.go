// Package guards is the in-process trust kernel — host-agnostic Go
// implementations of the bash scripts/guards/*.sh + scripts/hooks/*.sh.
//
// Each guard satisfies core.Guard and is invokable via
// `evolve guard <name>` (wired in cmd/evolve/cmd_guard.go in task #17).
package guards

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// Chain delegates to the ledger's hash-chain verifier. It's the in-
// process port of scripts/observability/verify-ledger-chain.sh exposed
// through the unified Guard interface so host hooks can shim to one
// shape: `exec evolve guard <name>`.
type Chain struct {
	ledger core.Ledger
}

// NewChain returns a Chain guard backed by ledger. The ledger may be
// nil only when callers do not invoke Decide (e.g. constructing for
// Name discovery).
func NewChain(ledger core.Ledger) *Chain {
	return &Chain{ledger: ledger}
}

// Name reports "chain".
func (c *Chain) Name() string { return "chain" }

// Decide verifies the ledger and reports Allow=true on success.
// Any Verify error (chain break, tip mismatch, read error, …) yields
// Allow=false with the underlying error message as Reason.
func (c *Chain) Decide(ctx context.Context, _ core.GuardInput) core.GuardDecision {
	if c.ledger == nil {
		return core.GuardDecision{Allow: false, Reason: "chain guard: no ledger configured"}
	}
	if err := c.ledger.Verify(ctx); err != nil {
		return core.GuardDecision{
			Allow:  false,
			Reason: fmt.Sprintf("ledger chain verify failed: %v", err),
		}
	}
	return core.GuardDecision{Allow: true}
}

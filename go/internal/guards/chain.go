// Package guards is the in-process trust kernel: the core.Guard implementations behind
// `evolve guard <name>` and the manifest of the protected pipeline control plane.
// See docs/architecture/packages/internal-guards.md.
package guards

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Chain allows only while the ledger's hash chain verifies.
type Chain struct {
	ledger core.Ledger
}

// NewChain returns a Chain guard backed by ledger; with a nil ledger, Decide denies.
func NewChain(ledger core.Ledger) *Chain {
	return &Chain{ledger: ledger}
}

// Name reports "chain".
func (c *Chain) Name() string { return "chain" }

// Decide verifies the ledger and denies on any Verify error, with the error as Reason.
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

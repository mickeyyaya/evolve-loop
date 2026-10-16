// Package guards is the in-process trust kernel: the core.Guard implementations behind
// `evolve guard <name>` and the manifest of the protected pipeline control plane.
// See docs/architecture/packages/internal-guards.md.
package guards

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type Chain struct {
	ledger core.Ledger
}

func NewChain(ledger core.Ledger) *Chain {
	return &Chain{ledger: ledger}
}

func (c *Chain) Name() string { return "chain" }

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

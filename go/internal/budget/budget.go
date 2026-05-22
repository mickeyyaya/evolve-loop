// Package budget tracks per-cycle + batch-cumulative LLM spend for the
// evolve-loop orchestrator. It is the Go port of the bash dispatch
// budget accounting at scripts/dispatch/evolve-loop-dispatch.sh and
// state.json:currentBatch.cycleAccruedCostUSD.
//
// The Meter honors three CLAUDE.md env-var contracts:
//
//	EVOLVE_MAX_BUDGET_USD       per-invocation override (default 999999)
//	EVOLVE_BATCH_BUDGET_CAP     batch cumulative cap   (default 20.00,
//	                            trip → DISPATCH_RC=4)
//	EVOLVE_BUILDER_COST_THRESHOLD   Build-phase guard (default 2.00)
//
// All three caps may be supplied independently via New() + setters;
// cap=0 means "unlimited" matching the bash defaults of "999999"-ish.
//
// The Meter is safe for concurrent use — plan §2 decision #11
// (multi-project parallel cycles from one process) shares one Meter
// across goroutines.
package budget

import (
	"errors"
	"fmt"
	"math"
	"sync"
)

// Sentinel errors. ErrBatchCapExceeded and ErrMaxBudgetExceeded are
// distinguished so the orchestrator can map them to the correct
// DISPATCH_RC value (4 for batch cap; 1 for per-invocation max).
var (
	ErrBatchCapExceeded  = errors.New("budget: batch cap exceeded")
	ErrMaxBudgetExceeded = errors.New("budget: per-invocation max exceeded")
)

// Meter accumulates LLM spend across one dispatcher invocation. It
// keeps three independent accumulators:
//
//	cycle    — reset every AdvanceCycle()
//	batch    — persists for the lifetime of the Meter
//	builder  — Build-phase-only; resets every AdvanceCycle()
type Meter struct {
	mu      sync.Mutex
	maxUSD  float64
	capUSD  float64
	thresh  float64
	cycle   float64
	batch   float64
	builder float64
}

// New constructs a Meter. Passing 0 for either cap disables that check
// (matches the EVOLVE_MAX_BUDGET_USD=999999 bash default semantics).
func New(maxUSD, batchCapUSD float64) *Meter {
	return &Meter{maxUSD: maxUSD, capUSD: batchCapUSD}
}

// SetBuilderThreshold configures the EVOLVE_BUILDER_COST_THRESHOLD
// guard. threshold=0 disables the check; matches the bash default
// where the guard is opt-in by setting a non-zero value.
func (m *Meter) SetBuilderThreshold(usd float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.thresh = usd
}

// Add records USD spent by an actor. The "build" actor additionally
// contributes to the builder-specific accumulator used by
// BuilderOver(). Returns a sentinel-wrapped error when either cap
// would be exceeded — the spend is still applied (operator logs read
// the post-trip Batch() value to decide kill scope).
func (m *Meter) Add(actor string, usd float64) error {
	if math.IsNaN(usd) || math.IsInf(usd, 0) {
		return fmt.Errorf("budget: refused non-finite spend %v from actor %q", usd, actor)
	}
	if usd < 0 {
		return fmt.Errorf("budget: refused negative spend %g from actor %q", usd, actor)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cycle += usd
	m.batch += usd
	if actor == "build" {
		m.builder += usd
	}
	if m.maxUSD > 0 && m.cycle > m.maxUSD {
		return fmt.Errorf("%w (cycle=%g, max=%g)", ErrMaxBudgetExceeded, m.cycle, m.maxUSD)
	}
	if m.capUSD > 0 && m.batch > m.capUSD {
		return fmt.Errorf("%w (batch=%g, cap=%g)", ErrBatchCapExceeded, m.batch, m.capUSD)
	}
	return nil
}

// AdvanceCycle resets the per-cycle and builder accumulators. Batch
// accumulator persists across cycles for the dispatcher invocation.
func (m *Meter) AdvanceCycle() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cycle = 0
	m.builder = 0
}

// Cycle returns USD spent so far in the current cycle.
func (m *Meter) Cycle() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cycle
}

// Batch returns USD spent cumulatively across the dispatcher invocation.
func (m *Meter) Batch() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.batch
}

// Builder returns USD spent by the Build phase in the current cycle.
func (m *Meter) Builder() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.builder
}

// BuilderOver reports whether the configured builder threshold has been
// exceeded. Returns false when no threshold is set.
func (m *Meter) BuilderOver() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.thresh > 0 && m.builder > m.thresh
}

// PercentOfBatchCap returns the percentage of the batch cap consumed.
// Returns 0 when the cap is unlimited; orchestrator uses this with the
// EVOLVE_CHECKPOINT_AT_PCT (default 95) trigger.
func (m *Meter) PercentOfBatchCap() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.capUSD <= 0 {
		return 0
	}
	return (m.batch / m.capUSD) * 100
}

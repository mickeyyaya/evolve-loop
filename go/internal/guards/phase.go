package guards

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Phase denies the in-process Agent tool during an active cycle, so phase agents go through the native bridge.
type Phase struct {
	storage core.Storage
	bypass  bool
}

// NewPhase returns a Phase guard that reads cycle state from s; bypass allows every call.
func NewPhase(s core.Storage, bypass bool) *Phase { return &Phase{storage: s, bypass: bypass} }

// Name reports "phase".
func (p *Phase) Name() string { return "phase" }

// Decide denies Agent while a cycle is active, and fails closed when cycle state cannot be read.
func (p *Phase) Decide(ctx context.Context, in core.GuardInput) core.GuardDecision {
	if p.bypass {
		return core.GuardDecision{Allow: true}
	}
	if in.ToolName != "Agent" {
		return core.GuardDecision{Allow: true}
	}
	if p.storage == nil {
		return core.GuardDecision{
			Allow:  false,
			Reason: "phase guard: storage not configured; refusing Agent invocation by default",
		}
	}
	cs, err := p.storage.ReadCycleState(ctx)
	if err != nil {
		return core.GuardDecision{
			Allow:  false,
			Reason: "phase guard: cycle-state read failed: " + err.Error(),
		}
	}
	if cs.CycleID != 0 {
		return core.GuardDecision{
			Allow: false,
			Reason: "Agent tool denied during cycle " +
				cs.Phase + " (use the native subagent bridge); pass --bypass to override in an emergency",
		}
	}
	return core.GuardDecision{Allow: true}
}

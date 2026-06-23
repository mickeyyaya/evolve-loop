package guards

import (
	"context"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// Phase blocks the Agent tool when a cycle is in progress (cycle-state.json
// has a non-empty cycle_id). Port of the phase-gate-precondition.sh
// agent-tool-during-cycle rule (one of several rules in the original;
// the rest land in Phase 2).
type Phase struct {
	storage core.Storage
	bypass  bool
}

func NewPhase(s core.Storage, bypass bool) *Phase { return &Phase{storage: s, bypass: bypass} }

func (p *Phase) Name() string { return "phase" }

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

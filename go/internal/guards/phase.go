package guards

import (
	"context"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

type Phase struct {
	storage core.Storage
	bypass  bool
	getenv  func(string) string
}

func NewPhase(s core.Storage, bypass bool) *Phase { return newPhaseWithEnv(s, bypass, os.Getenv) }

func newPhaseWithEnv(s core.Storage, bypass bool, getenv func(string) string) *Phase {
	return &Phase{storage: s, bypass: bypass, getenv: getenv}
}

func (p *Phase) Name() string { return "phase" }

func (p *Phase) Decide(ctx context.Context, in core.GuardInput) core.GuardDecision {
	persona := strField(in, "subagent_type")
	if p.bypass || !isSubagentTool(in.ToolName) || p.getenv(ipcenv.PhaseKey) != "" || !isPhasePersona(persona) {
		return core.GuardDecision{Allow: true}
	}
	if p.storage == nil {
		return core.GuardDecision{
			Allow:  false,
			Reason: "phase guard: storage not configured; refusing " + in.ToolName + " of the phase persona " + persona + " by default",
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
			Reason: in.ToolName + " of the phase persona " + persona + " denied outside a phase cell during cycle " +
				cs.Phase + " (phase agents start through the native bridge); pass --bypass to override in an emergency",
		}
	}
	return core.GuardDecision{Allow: true}
}

func isSubagentTool(name string) bool { return name == "Agent" || name == "Task" }

func isPhasePersona(subagentType string) bool {
	role := strings.TrimSpace(subagentType[strings.LastIndex(subagentType, ":")+1:])
	return strings.HasPrefix(strings.ToLower(role), "evolve-")
}

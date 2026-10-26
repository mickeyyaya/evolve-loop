package core

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/recoveryretro"
)

func (cr *cycleRun) writeShadowRetrospectives() {
	if floorStage(cr.req.ProjectRoot, "retrospectives", "retrospective writer", func(p policy.Policy) string { return p.RetrospectivesConfig().Stage }, cr.o.errOut()) == config.StageOff {
		return
	}
	if _, err := recoveryretro.WriteShadow(cr.req.ProjectRoot, cr.cs.WorkspacePath, cr.cycle, cr.state.CarryoverTodos); err != nil {
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN cycle %d: the shadow retrospective is not complete (non-fatal): %v\n", cr.cycle, err)
	}
}

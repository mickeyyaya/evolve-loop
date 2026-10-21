package core

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/recoveryretro"
)

func (cr *cycleRun) writeShadowRetrospectives() {
	if _, err := recoveryretro.WriteShadow(cr.req.ProjectRoot, cr.cs.WorkspacePath, cr.cycle, cr.state.CarryoverTodos); err != nil {
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN cycle %d: the shadow retrospective is not complete (non-fatal): %v\n", cr.cycle, err)
	}
}

package core

import (
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/recoveryretro"
)

func retrospectivesStage(projectRoot string, warn io.Writer) config.Stage {
	return floorStage(projectRoot, "retrospectives", "retrospective writer", func(p policy.Policy) string { return p.RetrospectivesConfig().Stage }, warn)
}

func (cr *cycleRun) writeShadowRetrospectives() {
	stage := retrospectivesStage(cr.req.ProjectRoot, cr.o.errOut())
	if stage == config.StageOff {
		return
	}
	var err error
	if stage == config.StageEnforce {
		_, err = recoveryretro.WriteInto(recoveryretro.PendingDir(cr.req.ProjectRoot), cr.req.ProjectRoot, cr.cs.WorkspacePath, cr.cycle, cr.state.CarryoverTodos)
	} else {
		_, err = recoveryretro.WriteShadow(cr.req.ProjectRoot, cr.cs.WorkspacePath, cr.cycle, cr.state.CarryoverTodos)
	}
	if err != nil {
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN cycle %d: the %s retrospective is not complete (non-fatal): %v\n", cr.cycle, stage, err)
	}
}

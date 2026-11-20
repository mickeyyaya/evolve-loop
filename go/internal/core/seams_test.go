package core

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

var (
	_ = (*cycleRun).recordAndBranch
	_ = (*cycleRun).selectNext
	_ = (*Orchestrator).registerMintedPhases
	_ = mintConfigsFrom
	_ = (*StateMachine).SpineSatisfiedUpTo
	_ = router.ClampPlanToFloorWith
)

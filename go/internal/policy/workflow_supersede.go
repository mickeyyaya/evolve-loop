package policy

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

type SupersedePolicy struct {
	Stage string `json:"stage,omitempty"`
}

type SupersedeConfig struct {
	Stage config.Stage
}

func (w *WorkflowPolicy) validateSupersede() error {
	if w == nil || w.Supersede == nil {
		return nil
	}
	stage := w.Supersede.Stage
	if _, ok := config.GateStage(stage); stage != "" && !ok {
		return fmt.Errorf("workflow.supersede.stage: %q is not 0, off, shadow or enforce", stage)
	}
	return nil
}

func (p Policy) supersedeConfig() SupersedeConfig {
	c := SupersedeConfig{Stage: config.StageShadow}
	if p.Workflow == nil || p.Workflow.Supersede == nil {
		return c
	}
	if set, ok := config.GateStage(p.Workflow.Supersede.Stage); ok {
		c.Stage = set
	}
	return c
}

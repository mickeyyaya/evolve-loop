package policy

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

type BuilderSelfCheckPolicy struct {
	Stage string `json:"stage,omitempty"`
}

type BuilderSelfCheckConfig struct {
	Stage config.Stage
}

func (w *WorkflowPolicy) validateBuilderSelfCheck() error {
	if w == nil || w.BuilderSelfCheck == nil {
		return nil
	}
	stage := w.BuilderSelfCheck.Stage
	if _, ok := config.GateStage(stage); stage != "" && !ok {
		return fmt.Errorf("workflow.builder_selfcheck.stage: %q is not 0, off, shadow or enforce", stage)
	}
	return nil
}

func (p Policy) builderSelfCheckConfig() BuilderSelfCheckConfig {
	c := BuilderSelfCheckConfig{Stage: config.StageShadow}
	if p.Workflow == nil || p.Workflow.BuilderSelfCheck == nil {
		return c
	}
	if set, ok := config.GateStage(p.Workflow.BuilderSelfCheck.Stage); ok {
		c.Stage = set
	}
	return c
}

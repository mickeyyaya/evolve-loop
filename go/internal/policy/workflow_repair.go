package policy

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

const defaultRepairFixabilityStage = "shadow"

type RepairPolicy struct {
	Fixability *RepairFixabilityPolicy `json:"fixability,omitempty"`
}

type RepairFixabilityPolicy struct {
	Stage string `json:"stage,omitempty"`
}

func (w *WorkflowPolicy) validateRepair() error {
	if w == nil || w.Repair == nil || w.Repair.Fixability == nil {
		return nil
	}
	if stage := w.Repair.Fixability.Stage; stage != "" {
		if _, ok := config.GateStage(stage); !ok {
			return fmt.Errorf("workflow.repair.fixability.stage: %q is not off, shadow or enforce", stage)
		}
	}
	return nil
}

func (p Policy) RepairFixabilityStage() config.Stage {
	stage, _ := config.GateStage(defaultRepairFixabilityStage)
	if p.Workflow == nil || p.Workflow.Repair == nil || p.Workflow.Repair.Fixability == nil {
		return stage
	}
	if set, ok := config.GateStage(p.Workflow.Repair.Fixability.Stage); ok {
		return set
	}
	return stage
}

func RepairFixabilityStageFor(projectRoot, who string, warn io.Writer) config.Stage {
	pol, err := Load(filepath.Join(paths.EvolveDirOf(projectRoot), paths.PolicyFile))
	if err != nil {
		fmt.Fprintf(warn, "[%s] WARN policy: workflow.repair.fixability: %v; the fixability check holds shadow\n", who, err)
		return config.StageShadow
	}
	return pol.RepairFixabilityStage()
}

package policy

import (
	"cmp"
	"fmt"
	"io"
	"path/filepath"
)

const (
	FixabilityOff     = "off"
	FixabilityShadow  = "shadow"
	FixabilityEnforce = "enforce"
)

type RepairPolicy struct {
	Fixability *FixabilityPolicy `json:"fixability,omitempty"`
}

type FixabilityPolicy struct {
	Stage string `json:"stage,omitempty"`
}

type RepairConfig struct {
	Fixability FixabilityConfig
}

type FixabilityConfig struct {
	Stage string
}

func (c FixabilityConfig) Enforced() bool {
	return c.Stage == FixabilityEnforce
}

func (p Policy) repairConfig() RepairConfig {
	c := RepairConfig{
		Fixability: FixabilityConfig{Stage: FixabilityShadow},
	}
	if p.Workflow == nil || p.Workflow.Repair == nil {
		return c
	}
	if p.Workflow.Repair.Fixability != nil {
		c.Fixability.Stage = cmp.Or(p.Workflow.Repair.Fixability.Stage, c.Fixability.Stage)
	}
	return c
}

func (w *WorkflowPolicy) validateRepair() error {
	if w == nil || w.Repair == nil {
		return nil
	}
	return w.Repair.validate()
}

func (r *RepairPolicy) validate() error {
	if r == nil || r.Fixability == nil {
		return nil
	}
	stage := r.Fixability.Stage
	if stage != "" && stage != FixabilityOff && stage != FixabilityShadow && stage != FixabilityEnforce {
		return fmt.Errorf("workflow.repair.fixability.stage must be %s, %s or %s, got %q", FixabilityOff, FixabilityShadow, FixabilityEnforce, stage)
	}
	return nil
}

func FixabilityFor(projectRoot, who string, warn io.Writer) FixabilityConfig {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		fmt.Fprintf(warn, "[%s] WARN policy: workflow.repair.fixability: %v; the fixability check holds shadow\n", who, err)
		return Policy{}.repairConfig().Fixability
	}
	return pol.repairConfig().Fixability
}

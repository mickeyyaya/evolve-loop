package policy

import (
	"errors"
	"fmt"
)

const (
	HealingStageOff    = "off"
	HealingStageShadow = "shadow"

	healingWindowHours = 72
	healingCycles      = 60
	healingMaxPerPass  = 2
)

type HealingPolicy struct {
	Detector     *HealingDetectorPolicy     `json:"detector,omitempty"`
	Orchestrator *HealingOrchestratorPolicy `json:"orchestrator,omitempty"`
}

type HealingOrchestratorPolicy struct {
	Stage      string `json:"stage,omitempty"`
	MaxPerPass int    `json:"max_per_pass,omitempty"`
}

func (p Policy) HealingOrchestrator() HealingOrchestratorPolicy {
	out := HealingOrchestratorPolicy{}
	if p.Healing != nil && p.Healing.Orchestrator != nil {
		out = *p.Healing.Orchestrator
	}
	if out.Stage == "" {
		out.Stage = HealingStageShadow
	}
	if out.MaxPerPass == 0 {
		out.MaxPerPass = healingMaxPerPass
	}
	return out
}

type HealingDetectorPolicy struct {
	Stage       string `json:"stage,omitempty"`
	WindowHours int    `json:"window_hours,omitempty"`
	Cycles      int    `json:"cycles,omitempty"`
}

func (p Policy) HealingDetector() HealingDetectorPolicy {
	out := HealingDetectorPolicy{}
	if p.Healing != nil && p.Healing.Detector != nil {
		out = *p.Healing.Detector
	}
	if out.Stage == "" {
		out.Stage = HealingStageShadow
	}
	if out.WindowHours == 0 {
		out.WindowHours = healingWindowHours
	}
	if out.Cycles == 0 {
		out.Cycles = healingCycles
	}
	return out
}

func (h *HealingPolicy) validate() error {
	if h == nil {
		return nil
	}
	if err := h.Orchestrator.validate(); err != nil {
		return err
	}
	if h.Detector == nil {
		return nil
	}
	d := h.Detector
	switch {
	case d.Stage == "enforce":
		return errors.New(`detector.stage "enforce" lands with the inbox item selfheal-action-catalog-validator-budgets (X14); the detector only records incidents, so use "shadow"`)
	case d.Stage != "" && d.Stage != HealingStageOff && d.Stage != HealingStageShadow:
		return fmt.Errorf(`detector.stage %q: the stages are "off" and "shadow"`, d.Stage)
	case d.WindowHours < 0:
		return fmt.Errorf("detector.window_hours %d: a window cannot be negative", d.WindowHours)
	case d.Cycles < 0:
		return fmt.Errorf("detector.cycles %d: a cycle count cannot be negative", d.Cycles)
	}
	return nil
}

func (o *HealingOrchestratorPolicy) validate() error {
	switch {
	case o == nil:
		return nil
	case o.Stage == "enforce":
		return errors.New(`orchestrator.stage "enforce" lands with the inbox item selfheal-action-catalog-validator-budgets (X14); the orchestrator agent only proposes, so use "shadow"`)
	case o.Stage != "" && o.Stage != HealingStageOff && o.Stage != HealingStageShadow:
		return fmt.Errorf(`orchestrator.stage %q: the stages are "off" and "shadow"`, o.Stage)
	case o.MaxPerPass < 0:
		return fmt.Errorf("orchestrator.max_per_pass %d: a count cannot be negative", o.MaxPerPass)
	}
	return nil
}

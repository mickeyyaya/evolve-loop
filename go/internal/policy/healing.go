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
)

type HealingPolicy struct {
	Detector *HealingDetectorPolicy `json:"detector,omitempty"`
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
	if h == nil || h.Detector == nil {
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

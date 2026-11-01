package policy

import (
	"errors"
	"fmt"
)

const (
	HealingStageOff     = "off"
	HealingStageShadow  = "shadow"
	HealingStageEnforce = "enforce"

	healingWindowHours       = 72
	healingCycles            = 60
	healingMaxPerPass        = 2
	healingRetryAfterHours   = 6
	healingBudgetWindowHours = 24
	healingT1Repairs         = 3
	healingT2Repairs         = 6
	healingBreakerFailures   = 2
	healingBreakerOpenHours  = 24
	healingEnforceItem       = "selfheal-repair-cycle-path"
)

type HealingPolicy struct {
	Detector     *HealingDetectorPolicy     `json:"detector,omitempty"`
	Orchestrator *HealingOrchestratorPolicy `json:"orchestrator,omitempty"`
	Validator    *HealingValidatorPolicy    `json:"validator,omitempty"`
	LiveReaction *HealingLiveReactionPolicy `json:"live_reaction,omitempty"`
}

type HealingOrchestratorPolicy struct {
	Stage           string `json:"stage,omitempty"`
	MaxPerPass      int    `json:"max_per_pass,omitempty"`
	RetryAfterHours int    `json:"retry_after_hours,omitempty"`
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
	if out.RetryAfterHours == 0 {
		out.RetryAfterHours = healingRetryAfterHours
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

type HealingValidatorPolicy struct {
	Stage            string `json:"stage,omitempty"`
	WindowHours      int    `json:"window_hours,omitempty"`
	T1Repairs        int    `json:"t1_repairs,omitempty"`
	T2Repairs        int    `json:"t2_repairs,omitempty"`
	BreakerFailures  int    `json:"breaker_failures,omitempty"`
	BreakerOpenHours int    `json:"breaker_open_hours,omitempty"`
}

func (p Policy) HealingValidator() HealingValidatorPolicy {
	out := HealingValidatorPolicy{}
	if p.Healing != nil && p.Healing.Validator != nil {
		out = *p.Healing.Validator
	}
	if out.Stage == "" {
		out.Stage = HealingStageShadow
	}
	for _, f := range []struct {
		value    *int
		fallback int
	}{
		{&out.WindowHours, healingBudgetWindowHours}, {&out.T1Repairs, healingT1Repairs}, {&out.T2Repairs, healingT2Repairs},
		{&out.BreakerFailures, healingBreakerFailures}, {&out.BreakerOpenHours, healingBreakerOpenHours},
	} {
		if *f.value == 0 {
			*f.value = f.fallback
		}
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
	if err := h.Validator.validate(); err != nil {
		return err
	}
	if err := h.LiveReaction.validate(); err != nil {
		return err
	}
	if h.Detector == nil {
		return nil
	}
	d := h.Detector
	switch {
	case d.Stage != "" && d.Stage != HealingStageOff && d.Stage != HealingStageShadow:
		return healingStageError("detector", d.Stage, "the detector only records incidents")
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
	case o.Stage != "" && o.Stage != HealingStageOff && o.Stage != HealingStageShadow:
		return healingStageError("orchestrator", o.Stage, "the orchestrator agent only proposes")
	case o.MaxPerPass < 0:
		return fmt.Errorf("orchestrator.max_per_pass %d: a count cannot be negative", o.MaxPerPass)
	case o.RetryAfterHours < 0:
		return fmt.Errorf("orchestrator.retry_after_hours %d: a backoff cannot be negative", o.RetryAfterHours)
	}
	return nil
}

func (v *HealingValidatorPolicy) validate() error {
	if v == nil {
		return nil
	}
	if v.Stage != "" && v.Stage != HealingStageOff && v.Stage != HealingStageShadow {
		return healingStageError("validator", v.Stage, "the validator only records its decisions")
	}
	for _, f := range []struct {
		name  string
		value int
	}{
		{"window_hours", v.WindowHours}, {"t1_repairs", v.T1Repairs}, {"t2_repairs", v.T2Repairs},
		{"breaker_failures", v.BreakerFailures}, {"breaker_open_hours", v.BreakerOpenHours},
	} {
		if f.value < 0 {
			return fmt.Errorf("validator.%s %d: a budget cannot be negative", f.name, f.value)
		}
	}
	return nil
}

func healingStageError(part, stage, why string) error {
	if stage == HealingStageEnforce {
		return errors.New(part + `.stage "enforce" lands with the inbox item ` + healingEnforceItem + ` (X16); ` + why + `, so use "shadow"`)
	}
	return fmt.Errorf(`%s.stage %q: the stages are "off" and "shadow"`, part, stage)
}

type HealingLiveReactionPolicy struct {
	Stage string `json:"stage,omitempty"`
}

func (p Policy) HealingLiveReaction() HealingLiveReactionPolicy {
	out := HealingLiveReactionPolicy{}
	if p.Healing != nil && p.Healing.LiveReaction != nil {
		out = *p.Healing.LiveReaction
	}
	if out.Stage == "" {
		out.Stage = HealingStageShadow
	}
	return out
}

func (l *HealingLiveReactionPolicy) validate() error {
	if l == nil {
		return nil
	}
	switch l.Stage {
	case "", HealingStageOff, HealingStageShadow, HealingStageEnforce:
		return nil
	}
	return fmt.Errorf(`live_reaction.stage %q: the stages are "off", "shadow" and "enforce"`, l.Stage)
}

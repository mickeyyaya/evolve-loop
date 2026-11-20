package policy

import (
	"errors"
	"fmt"
)

var ErrFleetAdaptive = errors.New("fleet.orchestrator.window.adaptive")

type AdaptiveStage string

const (
	AdaptiveStageShadow  AdaptiveStage = "shadow"
	AdaptiveStageEnforce AdaptiveStage = "enforce"
)

type FleetAdaptivePolicy struct {
	Stage              AdaptiveStage `json:"stage,omitempty"`
	LoadPerCoreHigh    float64       `json:"load_per_core_high,omitempty"`
	LoadPerCoreLow     float64       `json:"load_per_core_low,omitempty"`
	MemAvailableMinPct float64       `json:"mem_available_min_pct,omitempty"`
	SwapUsedMaxMB      float64       `json:"swap_used_max_mb,omitempty"`
	Step               int           `json:"step,omitempty"`
	DecreaseFactor     float64       `json:"decrease_factor,omitempty"`
	CooldownS          int           `json:"cooldown_s,omitempty"`
	IncreaseCooldownS  int           `json:"increase_cooldown_s,omitempty"`
	SampleS            int           `json:"sample_s,omitempty"`
	SmoothSamples      int           `json:"smooth_samples,omitempty"`
}

type AdaptiveConfig struct {
	Enabled           bool
	Stage             string
	LoadHigh          float64
	LoadLow           float64
	MemMinPct         float64
	SwapMaxMB         float64
	DecreaseFactor    float64
	Step              int
	CooldownS         int
	IncreaseCooldownS int
	SampleS           int
	SmoothSamples     int
}

const (
	defaultAdaptiveLoadHigh          = 1.5
	defaultAdaptiveLoadLow           = 1.0
	defaultAdaptiveMemMinPct         = 15
	defaultAdaptiveSwapMaxMB         = 4096
	defaultAdaptiveDecreaseFactor    = 0.5
	defaultAdaptiveStep              = 1
	defaultAdaptiveCooldownS         = 60
	defaultAdaptiveIncreaseCooldownS = 180
	defaultAdaptiveSampleS           = 15
	defaultAdaptiveSmoothSamples     = 3
	adaptiveMemPctCeiling            = 100
)

func (a *FleetAdaptivePolicy) validate() error {
	if a == nil {
		return nil
	}
	switch {
	case a.Stage != "" && a.Stage != AdaptiveStageShadow && a.Stage != AdaptiveStageEnforce:
		return fmt.Errorf("%w: stage %q is not \"shadow\" or \"enforce\"", ErrFleetAdaptive, a.Stage)
	case a.hasNegative():
		return fmt.Errorf("%w: no value may be negative", ErrFleetAdaptive)
	case a.MemAvailableMinPct >= adaptiveMemPctCeiling:
		return fmt.Errorf("%w: mem_available_min_pct must be under %d", ErrFleetAdaptive, adaptiveMemPctCeiling)
	case a.DecreaseFactor >= 1:
		return fmt.Errorf("%w: decrease_factor must be under 1", ErrFleetAdaptive)
	}
	if c := a.resolve(); c.LoadLow >= c.LoadHigh {
		return fmt.Errorf("%w: load_per_core_low %v must be under load_per_core_high %v", ErrFleetAdaptive, c.LoadLow, c.LoadHigh)
	}
	return nil
}

func (a *FleetAdaptivePolicy) hasNegative() bool {
	for _, v := range []float64{a.LoadPerCoreHigh, a.LoadPerCoreLow, a.MemAvailableMinPct, a.SwapUsedMaxMB, a.DecreaseFactor,
		float64(a.Step), float64(a.CooldownS), float64(a.IncreaseCooldownS), float64(a.SampleS), float64(a.SmoothSamples)} {
		if v < 0 {
			return true
		}
	}
	return false
}

func (a *FleetAdaptivePolicy) resolve() AdaptiveConfig {
	if a == nil {
		return AdaptiveConfig{}
	}
	return AdaptiveConfig{
		Enabled:           true,
		Stage:             orString(string(a.Stage), string(AdaptiveStageShadow)),
		LoadHigh:          orFloat(a.LoadPerCoreHigh, defaultAdaptiveLoadHigh),
		LoadLow:           orFloat(a.LoadPerCoreLow, defaultAdaptiveLoadLow),
		MemMinPct:         orFloat(a.MemAvailableMinPct, defaultAdaptiveMemMinPct),
		SwapMaxMB:         orFloat(a.SwapUsedMaxMB, defaultAdaptiveSwapMaxMB),
		DecreaseFactor:    orFloat(a.DecreaseFactor, defaultAdaptiveDecreaseFactor),
		Step:              orInt(a.Step, defaultAdaptiveStep),
		CooldownS:         orInt(a.CooldownS, defaultAdaptiveCooldownS),
		IncreaseCooldownS: orInt(a.IncreaseCooldownS, defaultAdaptiveIncreaseCooldownS),
		SampleS:           orInt(a.SampleS, defaultAdaptiveSampleS),
		SmoothSamples:     orInt(a.SmoothSamples, defaultAdaptiveSmoothSamples),
	}
}

func orString(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func orFloat(v, fallback float64) float64 {
	if v == 0 {
		return fallback
	}
	return v
}

func orInt(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}

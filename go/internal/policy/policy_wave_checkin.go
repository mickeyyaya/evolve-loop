package policy

import "github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"

const (
	defaultCheckinStallMinAgeMinutes   = 120
	defaultCheckinStallMinLiveLanes    = 4
	defaultCheckinMinShipsPerHour      = 0.25
	defaultCheckinAbnormalMinCycles    = 3
	defaultCheckinAbnormalMinEvents    = 15
	defaultCheckinStallLines           = 3
	defaultCheckinLoadSustainedMinutes = 15
	defaultCheckinStallWindowMinutes   = 60
	defaultCheckinLoadPerCoreLimit     = 3.5
)

type WaveCheckinPolicy struct {
	StallMinAgeMinutes   int     `json:"stall_min_age_minutes,omitempty"`
	StallMinLiveLanes    int     `json:"stall_min_live_lanes,omitempty"`
	MinShipsPerHour      float64 `json:"min_ships_per_hour,omitempty"`
	AbnormalMinCycles    int     `json:"abnormal_min_cycles,omitempty"`
	AbnormalMinEvents    int     `json:"abnormal_min_events,omitempty"`
	StallLines           int     `json:"stall_lines,omitempty"`
	StallWindowMinutes   int     `json:"stall_window_minutes,omitempty"`
	LoadPerCoreLimit     float64 `json:"load_per_core_limit,omitempty"`
	LoadSustainedMinutes int     `json:"load_sustained_minutes,omitempty"`
}

type WaveCheckinConfig struct {
	StallMinAgeMinutes   int
	StallMinLiveLanes    int
	MinShipsPerHour      float64
	AbnormalMinCycles    int
	AbnormalMinEvents    int
	StallLines           int
	StallWindowMinutes   int
	LoadSustainedMinutes int
	LoadPerCoreLimit     float64
	DiskFloorBytes       uint64
}

func (p Policy) WaveCheckinSettings() WaveCheckinConfig {
	var block WaveCheckinPolicy
	if p.Wave != nil && p.Wave.Checkin != nil {
		block = *p.Wave.Checkin
	}
	var gc gcpolicy.Policy
	if p.GC != nil {
		gc = *p.GC
	}
	return WaveCheckinConfig{
		StallMinAgeMinutes:   positiveOr(block.StallMinAgeMinutes, defaultCheckinStallMinAgeMinutes),
		StallMinLiveLanes:    positiveOr(block.StallMinLiveLanes, defaultCheckinStallMinLiveLanes),
		MinShipsPerHour:      positiveFloatOr(block.MinShipsPerHour, defaultCheckinMinShipsPerHour),
		AbnormalMinCycles:    positiveOr(block.AbnormalMinCycles, defaultCheckinAbnormalMinCycles),
		AbnormalMinEvents:    positiveOr(block.AbnormalMinEvents, defaultCheckinAbnormalMinEvents),
		StallLines:           positiveOr(block.StallLines, defaultCheckinStallLines),
		LoadSustainedMinutes: positiveOr(block.LoadSustainedMinutes, defaultCheckinLoadSustainedMinutes),
		StallWindowMinutes:   positiveOr(block.StallWindowMinutes, defaultCheckinStallWindowMinutes),
		LoadPerCoreLimit:     positiveFloatOr(block.LoadPerCoreLimit, defaultCheckinLoadPerCoreLimit),
		DiskFloorBytes:       gc.DiskFloorBytes(),
	}
}

func positiveFloatOr(v, fallback float64) float64 {
	if v > 0 {
		return v
	}
	return fallback
}

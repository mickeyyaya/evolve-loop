package gcpolicy

import "time"

type SentinelStage string

const (
	SentinelStageShadow  SentinelStage = "shadow"
	SentinelStageEnforce SentinelStage = "enforce"
)

type SentinelStageBlock struct {
	Stage SentinelStage `json:"stage,omitempty"`
}

type SentinelPolicy struct {
	IntervalS          int                `json:"interval_s,omitempty"`
	RepeatS            int                `json:"repeat_s,omitempty"`
	OrphanServersMax   int                `json:"orphan_servers_max,omitempty"`
	OrphanProcessesMax int                `json:"orphan_processes_max,omitempty"`
	HoldMaxS           int                `json:"hold_max_s,omitempty"`
	ReleaseBudgetS     int                `json:"release_budget_s,omitempty"`
	NoProgressBytes    int64              `json:"no_progress_bytes,omitempty"`
	Hold               SentinelStageBlock `json:"hold,omitempty"`
	Release            SentinelStageBlock `json:"release,omitempty"`
}

const (
	defaultSentinelIntervalS     = 60
	defaultSentinelRepeatS       = 900
	defaultSentinelOrphanMax     = 20
	defaultSentinelHoldMaxS      = 7200
	defaultSentinelReleaseBudget = 300
	defaultSentinelNoProgress    = int64(1 << 30)
)

func orInt(v, fallback int) int {
	if v <= 0 {
		return fallback
	}
	return v
}

func (s SentinelPolicy) Interval() time.Duration {
	return time.Duration(orInt(s.IntervalS, defaultSentinelIntervalS)) * time.Second
}

func (s SentinelPolicy) Repeat() time.Duration {
	return time.Duration(orInt(s.RepeatS, defaultSentinelRepeatS)) * time.Second
}

func (s SentinelPolicy) OrphanServersLimit() int {
	return orInt(s.OrphanServersMax, defaultSentinelOrphanMax)
}

func (s SentinelPolicy) OrphanProcessesLimit() int {
	return orInt(s.OrphanProcessesMax, defaultSentinelOrphanMax)
}

func (s SentinelPolicy) HoldMaxDuration() time.Duration {
	return time.Duration(orInt(s.HoldMaxS, defaultSentinelHoldMaxS)) * time.Second
}

func (s SentinelPolicy) ReleaseBudget() time.Duration {
	return time.Duration(orInt(s.ReleaseBudgetS, defaultSentinelReleaseBudget)) * time.Second
}

func (s SentinelPolicy) NoProgress() int64 {
	if s.NoProgressBytes <= 0 {
		return defaultSentinelNoProgress
	}
	return s.NoProgressBytes
}

func (s SentinelPolicy) HoldEnforced() bool { return s.Hold.Effective() == SentinelStageEnforce }

func (s SentinelPolicy) ReleaseEnforced() bool { return s.Release.Effective() == SentinelStageEnforce }

func (b SentinelStageBlock) Effective() SentinelStage {
	if b.Stage == SentinelStageEnforce {
		return SentinelStageEnforce
	}
	return SentinelStageShadow
}

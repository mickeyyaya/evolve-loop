package hostsentinel

import (
	"cmp"
	"fmt"
	"time"
)

type Stage string

const (
	StageOff     Stage = "off"
	StageShadow  Stage = "shadow"
	StageEnforce Stage = "enforce"
)

const bytesPerGiB = 1 << 30

const (
	defaultIntervalSeconds = 180
	defaultDiskLowGiB      = 20.0
	defaultFloorGiB        = 5.0
	defaultTmuxSessionsMax = 40
	defaultOrphansMax      = 20
	defaultLoadPerCPUMax   = 2.0
)

type Policy struct {
	Stage           string  `json:"stage,omitempty"`
	IntervalSeconds int     `json:"interval_seconds,omitempty"`
	DiskLowGiB      float64 `json:"disk_low_gib,omitempty"`
	FloorGiB        float64 `json:"floor_gib,omitempty"`
	TmuxSessionsMax int     `json:"tmux_sessions_max,omitempty"`
	OrphansMax      int     `json:"orphans_max,omitempty"`
	LoadPerCPUMax   float64 `json:"load_per_cpu_max,omitempty"`
}

type Config struct {
	Stage      Stage
	Interval   time.Duration
	FloorBytes uint64
	Limits     map[Metric]float64
}

func (p Policy) Config(goCacheMaxBytes int64) (Config, error) {
	stage := Stage(cmp.Or(p.Stage, string(StageShadow)))
	switch stage {
	case StageOff, StageShadow, StageEnforce:
	default:
		return Config{}, fmt.Errorf("host_sentinel.stage %q is not off, shadow or enforce", p.Stage)
	}
	return Config{
		Stage:      stage,
		Interval:   time.Duration(cmp.Or(p.IntervalSeconds, defaultIntervalSeconds)) * time.Second,
		FloorBytes: uint64(cmp.Or(p.FloorGiB, defaultFloorGiB) * bytesPerGiB),
		Limits: map[Metric]float64{
			MetricDiskFree: cmp.Or(p.DiskLowGiB, defaultDiskLowGiB) * bytesPerGiB,
			MetricGoCache:  float64(goCacheMaxBytes),
			MetricTmux:     float64(cmp.Or(p.TmuxSessionsMax, defaultTmuxSessionsMax)),
			MetricOrphans:  float64(cmp.Or(p.OrphansMax, defaultOrphansMax)),
			MetricLoad:     cmp.Or(p.LoadPerCPUMax, defaultLoadPerCPUMax),
		},
	}, nil
}

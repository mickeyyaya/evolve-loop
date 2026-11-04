package hostsentinel

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type Metric string

const (
	MetricDiskFree Metric = "disk_free_bytes"
	MetricGoCache  Metric = "go_cache_bytes"
	MetricTmux     Metric = "tmux_sessions"
	MetricOrphans  Metric = "orphans"
	MetricLoad     Metric = "load_per_cpu"
)

type Reading struct {
	Value float64 `json:"value"`
	Err   string  `json:"err,omitempty"`
}

func Measured(v float64, err error) Reading {
	if err != nil {
		return Reading{Err: err.Error()}
	}
	return Reading{Value: v}
}

type Sample struct {
	At       time.Time          `json:"at"`
	Readings map[Metric]Reading `json:"readings"`
}

type Breach struct {
	Code      signalcenter.Code `json:"code"`
	Metric    Metric            `json:"metric"`
	Value     float64           `json:"value"`
	Threshold float64           `json:"threshold"`
}

type check struct {
	metric Metric
	code   signalcenter.Code
	below  bool
	label  string
	unit   func(float64) string
}

var checks = []check{
	{MetricDiskFree, CodeDiskLow, true, "disk free", gibText},
	{MetricGoCache, CodeCacheHigh, false, "go cache", gibText},
	{MetricTmux, CodeTmuxHigh, false, "tmux", countText},
	{MetricOrphans, CodeOrphansHigh, false, "orphans", countText},
	{MetricLoad, CodeLoadHigh, false, "load per cpu", loadText},
}

func (k check) over(value, threshold float64) bool {
	if k.below {
		return value < threshold
	}
	return value > threshold
}

func Evaluate(s Sample, c Config) []Breach {
	var out []Breach
	for _, k := range checks {
		r, ok := s.Readings[k.metric]
		threshold := c.Limits[k.metric]
		if !ok || r.Err != "" || threshold <= 0 || !k.over(r.Value, threshold) {
			continue
		}
		out = append(out, Breach{Code: k.code, Metric: k.metric, Value: r.Value, Threshold: threshold})
	}
	return out
}

func ParseLoadavg(data []byte, cpus int) (float64, error) {
	fields := strings.Fields(string(data))
	if len(fields) == 0 || cpus <= 0 {
		return 0, errors.New("load average unreadable: no one-minute value or no cpu")
	}
	load, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("load average unreadable: %w", err)
	}
	return load / float64(cpus), nil
}

func gibText(v float64) string   { return fmt.Sprintf("%.2f GiB", v/bytesPerGiB) }
func countText(v float64) string { return strconv.FormatFloat(v, 'f', 0, 64) }
func loadText(v float64) string  { return strconv.FormatFloat(v, 'f', 2, 64) }

package pipelinerates

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

const (
	causeWindowPart = "/cause:"
	noneName        = "none"
)

type WaveView struct {
	Wave         int      `json:"wave"`
	Throughput   float64  `json:"pq6_per_day"`
	Utilization  *float64 `json:"pq7"`
	TopIdleShare *float64 `json:"pq8"`
	GateShare    *float64 `json:"pq9"`
	TopIdleCause string   `json:"top_idle_cause,omitempty"`
	TopFailClass string   `json:"top_fail_class,omitempty"`
}

func ReadWaveView(root string, now time.Time) (WaveView, error) {
	set, err := LoadWaveFlows(root)
	if err != nil {
		return WaveView{}, err
	}
	flows, err := set.WithSamples()
	if err != nil {
		return WaveView{}, err
	}
	cycles, err := LoadCycles(root)
	if err != nil {
		return WaveView{}, err
	}
	latest := flows[len(flows)-1:]
	view := WaveView{Wave: latest[0].Wave}
	for _, r := range CalendarThroughput(cycles, now) {
		view.Throughput += r.Interval.Value / calendarDays
	}
	view.Utilization = measured(SlotUtilization(latest)[0])
	view.GateShare = measured(GateShare(latest)[0])
	for _, r := range IdleCauseShare(latest) {
		if view.TopIdleShare == nil || r.Interval.Value > *view.TopIdleShare {
			view.TopIdleShare, view.TopIdleCause = measured(r), causeOf(r.Window)
		}
	}
	view.TopFailClass = topFailClass(cycles, view.Wave)
	return view, nil
}

func measured(r qualitypanel.Reading) *float64 {
	if r.N == 0 {
		return nil
	}
	v := r.Interval.Value
	return &v
}

func (v WaveView) Line() string {
	return fmt.Sprintf("health:  wave %d · PQ6 %.3f/day · top idle cause %s · top fail class %s · PQ7 %s · PQ8 %s · PQ9 %s",
		v.Wave, v.Throughput, orNone(v.TopIdleCause), orNone(v.TopFailClass), shown(v.Utilization), shown(v.TopIdleShare), shown(v.GateShare))
}

func shown(v *float64) string {
	if v == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.3f", *v)
}

func orNone(s string) string {
	if s == "" {
		return noneName
	}
	return s
}

func causeOf(window string) string {
	_, cause, _ := strings.Cut(window, causeWindowPart)
	return cause
}

func topFailClass(cycles []Cycle, wave int) string {
	counts := map[string]int{}
	for _, c := range cycles {
		if c.Wave != wave || c.Shipped {
			continue
		}
		for class, n := range c.FailClasses {
			counts[class] += n
		}
	}
	return topOf(counts)
}

func topOf(counts map[string]int) string {
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if counts[names[i]] != counts[names[j]] {
			return counts[names[i]] > counts[names[j]]
		}
		return names[i] < names[j]
	})
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

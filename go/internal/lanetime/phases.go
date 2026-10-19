package lanetime

import (
	"sort"
	"time"
)

type PhaseStat struct {
	Phase      string        `json:"phase"`
	Spans      int           `json:"spans"`
	Time       time.Duration `json:"time"`
	Share      float64       `json:"share"`
	Model      time.Duration `json:"model"`
	Host       time.Duration `json:"host"`
	Gates      int           `json:"gates"`
	GateTotal  time.Duration `json:"gate_total"`
	GateMedian time.Duration `json:"gate_median"`
}

type PhaseReport struct {
	Cycles   int           `json:"cycles"`
	Lane     time.Duration `json:"lane"`
	Gap      time.Duration `json:"gap"`
	GapShare float64       `json:"gap_share"`
	Phases   []PhaseStat   `json:"phases"`
}

type phaseAccum struct {
	stat  PhaseStat
	gates []time.Duration
}

func callsIn(s Span, calls []Call) []Call {
	var in []Call
	for _, c := range calls {
		if c.Phase == s.Phase && c.End.After(c.Start) && !c.Start.Before(s.Start) && !c.Start.After(s.End) {
			in = append(in, c)
		}
	}
	sort.Slice(in, func(i, j int) bool { return in[i].Start.Before(in[j].Start) })
	return in
}

func overlap(c Call, s Span) time.Duration {
	end := c.End
	if s.End.Before(end) {
		end = s.End
	}
	return end.Sub(c.Start)
}

func gatesAfter(s Span, calls []Call) []time.Duration {
	gates := make([]time.Duration, len(calls))
	for i, c := range calls {
		next := s.End
		if i+1 < len(calls) {
			next = calls[i+1].Start
		}
		gates[i] = max(next.Sub(c.End), 0)
	}
	return gates
}

func median(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), ds...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

func (a *phaseAccum) addSpan(s Span, calls []Call) {
	in := callsIn(s, calls)
	a.stat.Spans++
	a.stat.Time += s.End.Sub(s.Start)
	for _, c := range in {
		a.stat.Model += overlap(c, s)
	}
	for _, g := range gatesAfter(s, in) {
		a.gates = append(a.gates, g)
		a.stat.GateTotal += g
	}
}

func Phases(cycles []Cycle) PhaseReport {
	report := PhaseReport{Cycles: len(cycles)}
	accums := map[string]*phaseAccum{}
	for _, c := range cycles {
		var spanTime time.Duration
		for _, s := range c.Spans {
			a, ok := accums[s.Phase]
			if !ok {
				a = &phaseAccum{stat: PhaseStat{Phase: s.Phase}}
				accums[s.Phase] = a
			}
			a.addSpan(s, c.Calls)
			spanTime += s.End.Sub(s.Start)
		}
		report.Lane += c.Lane()
		report.Gap += max(c.Lane()-spanTime, 0)
	}
	report.GapShare = ratio(float64(report.Gap), float64(report.Lane))
	for _, a := range accums {
		a.stat.Model = min(a.stat.Model, a.stat.Time)
		a.stat.Host = a.stat.Time - a.stat.Model
		a.stat.Share = ratio(float64(a.stat.Time), float64(report.Lane))
		a.stat.Gates = len(a.gates)
		a.stat.GateMedian = median(a.gates)
		report.Phases = append(report.Phases, a.stat)
	}
	sort.Slice(report.Phases, func(i, j int) bool {
		if report.Phases[i].Time != report.Phases[j].Time {
			return report.Phases[i].Time > report.Phases[j].Time
		}
		return report.Phases[i].Phase < report.Phases[j].Phase
	})
	return report
}

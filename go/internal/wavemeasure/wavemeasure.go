package wavemeasure

import (
	"sort"
	"time"
)

const (
	FieldWidthSized = "width_sized"
	CodeAdmitWait   = "LOOP_ORCH_ADMIT_WAIT"
)

type Lane struct {
	Start    time.Time
	End      time.Time
	Landed   bool
	Conflict bool
}

type Facts struct {
	Start          time.Time
	End            time.Time
	WidthSized     int
	Lanes          []Lane
	AdmissionWaits int
}

type Measures struct {
	WidthSized     int     `json:"width_sized"`
	WidthRealized  int     `json:"width_realized"`
	Utilization    float64 `json:"utilization"`
	ConflictRate   float64 `json:"conflict_rate"`
	Conflicts      int     `json:"conflicts"`
	Landed         int     `json:"landed"`
	AdmissionWaits int     `json:"admission_waits"`
}

func Compute(f Facts, now time.Time) Measures {
	m := Measures{WidthSized: f.WidthSized, AdmissionWaits: f.AdmissionWaits}
	for _, l := range f.Lanes {
		if l.Landed {
			m.Landed++
			if l.Conflict {
				m.Conflicts++
			}
		}
	}
	if m.Landed > 0 {
		m.ConflictRate = float64(m.Conflicts) / float64(m.Landed)
	}
	start, end := f.Start, orNow(f.End, now)
	if !end.After(start) {
		return m
	}
	spans := clip(f.Lanes, start, end, now)
	runTime := end.Sub(start)
	if f.WidthSized > 0 {
		m.Utilization = float64(busy(spans)) / (float64(f.WidthSized) * float64(runTime))
	}
	m.WidthRealized = weightedMedian(widthTime(spans, start, end), runTime)
	return m
}

type span struct{ from, to time.Time }

func orNow(t, now time.Time) time.Time {
	if t.IsZero() {
		return now
	}
	return t
}

func clip(lanes []Lane, start, end, now time.Time) []span {
	var out []span
	for _, l := range lanes {
		if l.Start.IsZero() {
			continue
		}
		from, to := maxTime(l.Start, start), minTime(orNow(l.End, now), end)
		if to.After(from) {
			out = append(out, span{from: from, to: to})
		}
	}
	return out
}

func busy(spans []span) time.Duration {
	var total time.Duration
	for _, s := range spans {
		total += s.to.Sub(s.from)
	}
	return total
}

type edge struct {
	at    time.Time
	delta int
}

func widthTime(spans []span, start, end time.Time) map[int]time.Duration {
	edges := []edge{{at: end}}
	for _, s := range spans {
		edges = append(edges, edge{at: s.from, delta: 1}, edge{at: s.to, delta: -1})
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].at.Before(edges[j].at) })
	out := map[int]time.Duration{}
	width, last := 0, start
	for _, e := range edges {
		out[width] += e.at.Sub(last)
		width, last = width+e.delta, e.at
	}
	return out
}

func weightedMedian(byWidth map[int]time.Duration, total time.Duration) int {
	widths := make([]int, 0, len(byWidth))
	for w := range byWidth {
		widths = append(widths, w)
	}
	sort.Ints(widths)
	var seen time.Duration
	for _, w := range widths {
		seen += byWidth[w]
		if 2*seen >= total {
			return w
		}
	}
	return 0
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

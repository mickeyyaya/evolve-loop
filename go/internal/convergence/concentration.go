package convergence

import (
	"maps"
	"slices"
)

type hotspot struct {
	component   string
	share, prev float64
	fired       bool
}

type windowTop struct {
	component string
	share     float64
}

func (v view) concentration() hotspot {
	cur, curDefined := v.windowTop(v.in.Round)
	prev, prevDefined := v.windowTop(v.in.Round - 1)
	threshold := v.in.Config.ConcentrationThreshold
	return hotspot{
		component: cur.component, share: cur.share, prev: prev.share,
		fired: curDefined && prevDefined && cur.share >= threshold && prev.share >= threshold && cur.component == prev.component,
	}
}

func (v view) windowTop(end int) (windowTop, bool) {
	start := end - v.in.Config.ConcentrationWindow + 1
	if start < 0 {
		return windowTop{}, false
	}
	counts, seen, total := map[string]int{}, map[string]bool{}, 0
	for _, j := range v.in.Rounds[start : end+1] {
		for _, f := range j.Findings {
			component := componentOf(f)
			if f.Status != StatusOpen || !f.Severity.atLeast(SeverityLow) || component == unknownComponent || seen[f.ID] {
				continue
			}
			seen[f.ID] = true
			counts[component]++
			total++
		}
	}
	if total < v.in.Config.ConcentrationMinFindings {
		return windowTop{}, false
	}
	top := topComponent(counts)
	return windowTop{component: top, share: float64(counts[top]) / float64(total)}, true
}

func topComponent(counts map[string]int) string {
	names := slices.Sorted(maps.Keys(counts))
	top := names[0]
	for _, name := range names[1:] {
		if counts[name] > counts[top] {
			top = name
		}
	}
	return top
}

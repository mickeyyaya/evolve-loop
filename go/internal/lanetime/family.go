package lanetime

import (
	"sort"
	"time"
)

const DefaultBuilds = 20

type FamilyLane struct {
	Family  string        `json:"family"`
	Builds  int           `json:"builds"`
	Passes  int           `json:"passes"`
	Lane    time.Duration `json:"lane"`
	PerPass time.Duration `json:"per_pass"`
}

func LaneTimeToPass(cycles []Cycle, last int) []FamilyLane {
	ordered := append([]Cycle(nil), cycles...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	byFamily := map[string][]Cycle{}
	for _, c := range ordered {
		if family := c.BuildFamily(); family != "" && c.Open == 0 && c.Lane() > 0 {
			byFamily[family] = append(byFamily[family], c)
		}
	}
	var out []FamilyLane
	for family, builds := range byFamily {
		if over := len(builds) - last; last > 0 && over > 0 {
			builds = builds[over:]
		}
		out = append(out, tally(family, builds))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Family < out[j].Family })
	return out
}

func tally(family string, builds []Cycle) FamilyLane {
	f := FamilyLane{Family: family, Builds: len(builds)}
	for _, c := range builds {
		f.Lane += c.Lane()
		if c.Passed() {
			f.Passes++
		}
	}
	if f.Passes > 0 {
		f.PerPass = f.Lane / time.Duration(f.Passes)
	}
	return f
}

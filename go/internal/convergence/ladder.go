package convergence

const (
	causeSchedule     = "schedule"
	causeMarginalGain = "marginal-gain"
	causeOscillation  = "oscillation"
	causeFinalRound   = "final-round"
)

type ladder struct {
	view
	budget int
	rungs  []int
	causes []string
}

func newLadder(in Input) ladder {
	rules := rulesByLoop[in.Loop]
	l := ladder{view: view{in: in, rules: rules}, budget: rules.budget(in.Config), rungs: []int{0}, causes: []string{causeSchedule}}
	for k := 0; k < in.Round; k++ {
		next, cause := l.rungAfter(k)
		l.rungs = append(l.rungs, next)
		l.causes = append(l.causes, cause)
	}
	return l
}

func (l ladder) rungAfter(k int) (int, string) {
	switch {
	case k == 0:
		return schedule(1, l.budget), causeSchedule
	case l.rungs[k] >= 2:
		return 3, causeFinalRound
	case oscillates(l.in.Rounds[:k+1]):
		return 3, causeOscillation
	case !l.in.Rounds[k].Reentry && l.gainOf(k).spent():
		return 2, causeMarginalGain
	}
	return schedule(k+1, l.budget), causeSchedule
}

func schedule(round, budget int) int {
	switch {
	case round >= budget:
		return 2
	case round >= 2:
		return 1
	}
	return 0
}

func (l ladder) barOf(rung int) Severity {
	if l.rules.raisesBar && rung >= 2 {
		return Severity(l.in.Config.RaisedBlockingBar)
	}
	return Severity(l.in.Config.BaseBlockingBar)
}

type gain struct {
	repairEffect
	bar            Severity
	massPrev, mass float64
}

func (g gain) noProgress() bool { return g.mass >= g.massPrev }

func (g gain) damaging() bool { return g.damage >= g.repairs }

func (g gain) spent() bool { return g.damaging() || g.noProgress() }

func (l ladder) gainOf(k int) gain {
	g := gain{repairEffect: l.effectOf(k), bar: l.barOf(l.rungs[k])}
	g.massPrev, g.mass = l.mass(k-1, g.bar), l.mass(k, g.bar)
	return g
}

func oscillates(js []Judgment) bool {
	return reopenedTwice(js) || fingerprintBounced(js)
}

func reopenedTwice(js []Judgment) bool {
	last, reopenings := map[string]Status{}, map[string]int{}
	for _, j := range js {
		for _, f := range j.Findings {
			if last[f.ID] == StatusFixed && f.Status == StatusOpen {
				reopenings[f.ID]++
			}
			last[f.ID] = f.Status
		}
	}
	for _, n := range reopenings {
		if n >= 2 {
			return true
		}
	}
	return false
}

func fingerprintBounced(js []Judgment) bool {
	firstEdge := map[string]string{}
	for _, j := range js {
		if j.Fingerprint == "" {
			continue
		}
		edge, seen := firstEdge[j.Fingerprint]
		if seen && edge != j.Edge {
			return true
		}
		if !seen {
			firstEdge[j.Fingerprint] = j.Edge
		}
	}
	return false
}

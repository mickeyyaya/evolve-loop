package convergence

import "slices"

func (c decider) exitScope(strict []Finding) Decision {
	judge, missed := c.adjudicatingJudge(strict)
	if judge != (TierEffort{}) {
		return Decision{Rung: 3, Action: ActionAdjudicate, BlockingBar: c.bar, JudgeRaise: judge,
			LandRound: c.keepBest(c.bar, nil), Redesign: c.redesign(scopeExit{})}
	}
	d := c.changeScope(strict)
	d.Reasons = slices.Concat(missed, d.Reasons)
	return d
}

func (c decider) adjudicatingJudge(strict []Finding) (TierEffort, []string) {
	if !c.rules.adjudicates || !hasSeverity(strict, SeverityHigh) || hasSeverity(strict, SeverityCritical) {
		return TierEffort{}, nil
	}
	return c.raiseTo(roleJudge, c.in.Judge, topTier())
}

func (c decider) changeScope(strict []Finding) Decision {
	if hasSeverity(strict, SeverityCritical) {
		return c.stop("")
	}
	if component, ok := c.splittable(strict); ok {
		if c.rules.split == splitStopsWithContinuation {
			return c.stop(component)
		}
		return c.landing(Decision{Rung: 3, Action: ActionSplit}, scopeExit{split: component})
	}
	if components, ok := c.acceptable(strict); ok {
		return c.landing(Decision{Rung: 3, Action: ActionAcceptWithLimits}, scopeExit{accepted: components})
	}
	return c.stop("")
}

func (c decider) splittable(strict []Finding) (string, bool) {
	if c.rules.split == splitNever {
		return "", false
	}
	component := componentOf(strict[0])
	for _, f := range strict[1:] {
		if componentOf(f) != component {
			return "", false
		}
	}
	return component, c.in.Components[component].Separable
}

func (c decider) acceptable(strict []Finding) ([]string, bool) {
	if !c.rules.accepts {
		return nil, false
	}
	var components []string
	for _, f := range strict {
		component := componentOf(f)
		if c.in.Components[component].FailSafeCertificate == "" {
			return nil, false
		}
		if !slices.Contains(components, component) {
			components = append(components, component)
		}
	}
	slices.Sort(components)
	return components, true
}

func (c decider) stop(split string) Decision {
	exit := scopeExit{split: split}
	keep := c.keepBest(c.bar, exit.exempt())
	file := c.openIDs(keep)
	return Decision{Rung: 3, Action: ActionStop, BlockingBar: c.bar, File: file, SplitComponent: split,
		LandRound: keep, Redesign: c.redesign(exit), Reasons: c.stopReasons(split, len(file), keep)}
}

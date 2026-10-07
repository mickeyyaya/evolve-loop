package convergence

import "slices"

const (
	roleFixer = "fixer"
	roleJudge = "judge"
)

type scopeExit struct {
	split    string
	accepted []string
}

func (s scopeExit) exempt() []string {
	if s.split != "" {
		return []string{s.split}
	}
	return s.accepted
}

type decider struct {
	ladder
	next  int
	cause string
	bar   Severity
	hot   hotspot
}

func Decide(in Input) Decision {
	l := newLadder(in)
	next, cause := l.rungAfter(in.Round)
	c := decider{ladder: l, next: next, cause: cause, bar: l.barOf(next), hot: l.concentration()}
	d := c.choose()
	d.Reasons = slices.Concat([]string{c.rungReason(d)}, c.roundGainReasons(), c.concentrationReasons(), d.Reasons)
	return d
}

func (c decider) choose() Decision {
	exit := c.earlyExit()
	strict := c.strict(c.in.Round, c.bar, exit.exempt())
	switch {
	case len(strict) == 0:
		return c.land(exit)
	case c.next == 3:
		return c.exitScope(strict)
	}
	return c.proceed(strict, exit)
}

func (c decider) earlyExit() scopeExit {
	if !c.hot.fired || c.next == 3 {
		return scopeExit{}
	}
	strict := c.strict(c.in.Round, c.bar, nil)
	inHot := slices.ContainsFunc(strict, func(f Finding) bool { return componentOf(f) == c.hot.component })
	if !inHot || hasSeverity(strict, SeverityCritical) {
		return scopeExit{}
	}
	facts := c.in.Components[c.hot.component]
	switch {
	case c.rules.split == splitUnstages && facts.Separable:
		return scopeExit{split: c.hot.component}
	case c.rules.accepts && facts.FailSafeCertificate != "":
		return scopeExit{accepted: []string{c.hot.component}}
	}
	return scopeExit{}
}

func (c decider) land(exit scopeExit) Decision {
	rung, _ := c.landRung()
	return c.landing(Decision{Rung: rung, Action: ActionLand}, exit)
}

func (c decider) landing(d Decision, exit scopeExit) Decision {
	keep := c.keepBest(c.bar, exit.exempt())
	if c.hasOpenCritical(keep) {
		return c.stop("")
	}
	d.BlockingBar, d.LandRound = c.bar, keep
	return c.settle(d, keep, exit)
}

func (c decider) landRung() (int, string) {
	current := c.rungs[c.in.Round]
	if c.barOf(c.next) != c.barOf(current) {
		return 2, c.cause
	}
	return current, c.causes[c.in.Round]
}

func (c decider) proceed(strict []Finding, exit scopeExit) Decision {
	d := Decision{Rung: c.next, Action: ActionContinue, BlockingBar: c.bar, VerifyOnly: c.rules.verifyOnly,
		FreshContext: c.rules.freshContext && c.next == 2, LandRound: c.keepBest(c.bar, exit.exempt())}
	if c.feedbackChangeDue() {
		var fixerMissed, judgeMissed []string
		d.FixerRaise, fixerMissed = c.raiseOneTier(roleFixer, c.in.Fixer, c.rules.fixerRaise)
		d.JudgeRaise, judgeMissed = c.raiseOneTier(roleJudge, c.in.Judge, c.rules.judgeRaise && hasReasoningBlocker(strict))
		d.Reasons = slices.Concat(fixerMissed, judgeMissed)
	}
	return c.settle(d, c.in.Round, exit)
}

func (c decider) feedbackChangeDue() bool {
	return (c.next == 1 || c.next == 2) && !slices.Contains(c.rungs[1:], 1)
}

func (c decider) raiseOneTier(role string, from TierEffort, wanted bool) (TierEffort, []string) {
	if !wanted {
		return TierEffort{}, nil
	}
	return c.raiseTo(role, from, c.in.Headroom.tierAbove(from.Family, from.Tier))
}

func (c decider) raiseTo(role string, from TierEffort, tier string) (TierEffort, []string) {
	if raised, ok := c.in.Headroom.raise(from, tier); ok {
		return raised, nil
	}
	return TierEffort{}, []string{c.noHeadroomReason(role, from, tier)}
}

func (c decider) settle(d Decision, k int, exit scopeExit) Decision {
	filed := c.filedIn(k, exit.split)
	d.SplitComponent = exit.split
	d.Defer = append(c.acceptedIDs(k, exit.accepted), c.deferrals(k, d.Rung, exit.split)...)
	d.File = idsOf(filed)
	d.Redesign = c.redesign(exit)
	d.Reasons = slices.Concat(d.Reasons, c.settleReasons(filed, exit, len(d.Defer)))
	return d
}

func (c decider) acceptedIDs(k int, accepted []string) []string {
	var ids []string
	for _, f := range c.strict(k, c.bar, nil) {
		if slices.Contains(accepted, componentOf(f)) {
			ids = append(ids, f.ID)
		}
	}
	return ids
}

func (c decider) deferrals(k, rung int, split string) []string {
	if !c.rules.defers || rung < 2 {
		return nil
	}
	var ids []string
	for _, f := range c.in.Rounds[k].Findings {
		if c.unresolved(f) && f.Severity.atLeast(SeverityLow) && !f.Severity.atLeast(c.bar) && componentOf(f) != split {
			ids = append(ids, f.ID)
		}
	}
	return ids
}

func (c decider) filedIn(k int, split string) []Finding {
	var filed []Finding
	for _, f := range c.in.Rounds[k].Findings {
		if c.filedByRule(f) && componentOf(f) != split {
			filed = append(filed, f)
		}
	}
	return filed
}

func (c decider) redesign(exit scopeExit) []string {
	components := slices.Clone(exit.accepted)
	if c.hot.fired && c.hot.component != exit.split && !slices.Contains(components, c.hot.component) {
		components = append(components, c.hot.component)
	}
	slices.Sort(components)
	return components
}

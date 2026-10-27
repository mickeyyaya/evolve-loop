package retryladder

import (
	"cmp"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type Rung string

const (
	R0 Rung = "R0"
	R1 Rung = "R1"
	R2 Rung = "R2"
	R3 Rung = "R3"
)

type Action string

const (
	ActionRaise Action = "raise"
	ActionRetry Action = "retry"
	ActionStop  Action = "stop"
)

const (
	SignalNoHeadroom = "LADDER_NO_HEADROOM"
	SignalStopSpec   = "LADDER_STOP_SPEC"
)

const (
	DefaultMaxTier   = "deep"
	DefaultMaxEffort = "high"
)

type Bounds struct {
	MaxTier   string
	MaxEffort string
}

func (b Bounds) Resolved() Bounds {
	return Bounds{MaxTier: cmp.Or(b.MaxTier, DefaultMaxTier), MaxEffort: cmp.Or(b.MaxEffort, DefaultMaxEffort)}
}

type Input struct {
	Rung     Rung
	Earlier  []string
	Verdict  Verdict
	Base     convergence.TierEffort
	Chain    []string
	Headroom convergence.HeadroomTable
	Bounds   Bounds
	Critic   Rung
}

type Decision struct {
	Action     Action
	Rung       Rung
	Class      Class
	Target     convergence.TierEffort
	Signals    []string
	CriticRung Rung
}

func Next(in Input) Decision {
	d := Decision{Action: ActionStop, Rung: in.Rung, Class: in.Verdict.Class, CriticRung: in.Critic}
	b := newBounded(in)
	switch in.Verdict.Class {
	case Infra, Quota:
		d.Action = ActionRetry
	case Capability:
		return b.capability(d)
	case Unknown:
		if in.Rung == R0 {
			return d.raiseTo(R1, b.sameTierTarget())
		}
	}
	return d
}

func (d Decision) raiseTo(rung Rung, target convergence.TierEffort) Decision {
	d.Action, d.Rung, d.Target = ActionRaise, rung, target
	return d
}

type bounded struct {
	in        Input
	tierCap   int
	effortCap int
}

func newBounded(in Input) bounded {
	resolved := in.Bounds.Resolved()
	return bounded{in: in, tierCap: tierIndex(resolved.MaxTier), effortCap: effortIndex(resolved.MaxEffort)}
}

func tierIndex(tier string) int { return slices.Index(modelcatalog.CanonicalTiers, tier) }

func effortIndex(effort string) int { return slices.Index(policy.EffortLevels(), effort) }

func effortAt(i int) string {
	if i < 0 {
		return ""
	}
	return policy.EffortLevels()[i]
}

func (b bounded) capability(d Decision) Decision {
	repeated := b.in.Verdict.Fingerprint != "" && slices.Contains(b.in.Earlier, b.in.Verdict.Fingerprint)
	switch b.in.Rung {
	case R0:
		if _, hasStep := b.effortStep(); hasStep && !repeated {
			return d.raiseTo(R1, b.sameTierTarget())
		}
		return b.climbOrStayOnTier(d)
	case R1:
		if repeated {
			return d.raiseTo(R3, b.builderSeat())
		}
		return b.climbOrSkipToR3(d)
	case R2:
		return d.raiseTo(R3, b.builderSeat())
	case R3:
		if repeated {
			d.Class, d.Signals = Spec, []string{SignalStopSpec}
		}
	}
	d.Action = ActionStop
	return d
}

func (b bounded) climbOrStayOnTier(d Decision) Decision {
	if target, found, blockedByHeadroom := b.tierClimb(); found {
		return d.raiseTo(R2, target)
	} else if blockedByHeadroom {
		d.Signals = []string{SignalNoHeadroom}
	}
	return d.raiseTo(R1, b.sameTierTarget())
}

func (b bounded) climbOrSkipToR3(d Decision) Decision {
	target, found, blockedByHeadroom := b.tierClimb()
	if found {
		return d.raiseTo(R2, target)
	}
	if blockedByHeadroom {
		d.Signals = []string{SignalNoHeadroom}
	}
	return d.raiseTo(R3, b.builderSeat())
}

func (b bounded) builderSeat() convergence.TierEffort {
	base := b.in.Base
	family := base.Family
	for _, other := range b.in.Chain {
		if _, ok := b.in.Headroom[other][base.Tier]; ok && other != base.Family {
			family = other
			break
		}
	}
	seat, ok := b.in.Headroom[family][base.Tier]
	model := base.Model
	if ok {
		model = seat.Model
	}
	return convergence.TierEffort{Family: family, Tier: base.Tier, Model: model, Effort: b.climbEffort(seat.Effort)}
}

func (b bounded) effortStep() (string, bool) {
	current := effortIndex(b.in.Base.Effort)
	if current < 0 || current+1 > b.effortCap {
		return "", false
	}
	return effortAt(current + 1), true
}

func (b bounded) sameTierTarget() convergence.TierEffort {
	target := b.in.Base
	if step, ok := b.effortStep(); ok {
		target.Effort = step
	}
	return target
}

func (b bounded) tierClimb() (convergence.TierEffort, bool, bool) {
	next := tierIndex(b.in.Base.Tier) + 1
	if tierIndex(b.in.Base.Tier) < 0 || next > b.tierCap {
		return convergence.TierEffort{}, false, false
	}
	tier := modelcatalog.CanonicalTiers[next]
	for _, family := range b.in.Chain {
		if seat, ok := b.in.Headroom[family][tier]; ok {
			return convergence.TierEffort{Family: family, Tier: tier, Model: seat.Model, Effort: b.climbEffort(seat.Effort)}, true, false
		}
	}
	return convergence.TierEffort{}, false, true
}

func (b bounded) climbEffort(tierDefault string) string {
	raised := b.sameTierTarget().Effort
	larger := raised
	if effortIndex(tierDefault) > effortIndex(raised) {
		larger = tierDefault
	}
	ceiling := max(b.effortCap, effortIndex(b.in.Base.Effort))
	if effortIndex(larger) > ceiling {
		return effortAt(ceiling)
	}
	return larger
}

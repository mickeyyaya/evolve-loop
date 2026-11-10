package lifecycle

type Round struct {
	Try            int
	CarryFindings  bool
	RaiseEffort    bool
	ChangeStrategy bool
}

func (r Round) Plan() string {
	if r.ChangeStrategy {
		return "fresh-deep-plan"
	}
	return "raise-with-findings"
}

func PlanRound(failures, ceiling int) Round {
	try := failures + 1
	carry := failures > 0
	raise := failures > 0
	changeStrategy := (failures >= 2) || (ceiling > 0 && try >= ceiling)
	return Round{
		Try:            try,
		CarryFindings:  carry,
		RaiseEffort:    raise,
		ChangeStrategy: changeStrategy,
	}
}

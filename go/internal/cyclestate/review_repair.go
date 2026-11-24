package cyclestate

type RoundSnapshot struct {
	Observed bool              `json:"observed,omitempty"`
	GapMass  float64           `json:"gap_mass"`
	Statuses map[string]string `json:"statuses,omitempty"`
}

type ReviewRepair struct {
	Rounds []RoundSnapshot `json:"rounds,omitempty"`
}

func (r *ReviewRepair) WithRound(round int, snapshot RoundSnapshot) *ReviewRepair {
	if round < 1 {
		return r
	}
	history := make([]RoundSnapshot, round-1, round)
	if r != nil {
		copy(history, r.Rounds)
	}
	return &ReviewRepair{Rounds: append(history, snapshot)}
}

func (r *ReviewRepair) Snapshots() []RoundSnapshot {
	if r == nil {
		return nil
	}
	return append([]RoundSnapshot(nil), r.Rounds...)
}

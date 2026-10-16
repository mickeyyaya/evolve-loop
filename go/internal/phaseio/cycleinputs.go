package phaseio

import "unicode/utf8"

const (
	MaxFieldBytes    = 16 * 1024
	TruncationMarker = "\n...[truncated]"
)

func CapField(s string) string {
	if len(s) <= MaxFieldBytes {
		return s
	}
	cut := MaxFieldBytes - len(TruncationMarker)
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + TruncationMarker
}

type CycleInputsInit struct {
	Goal            string
	Strategy        string
	CommitMessage   string
	FleetScope      string
	ChallengeToken  string
	PreviousVerdict string
	Carryover       string
}

type CycleInputs struct {
	goal            string
	strategy        string
	commitMessage   string
	fleetScope      string
	challengeToken  string
	previousVerdict string
	carryover       string
}

func NewCycleInputs(init CycleInputsInit) CycleInputs {
	return CycleInputs{
		goal:            init.Goal,
		strategy:        init.Strategy,
		commitMessage:   init.CommitMessage,
		fleetScope:      init.FleetScope,
		challengeToken:  init.ChallengeToken,
		previousVerdict: CapField(init.PreviousVerdict),
		carryover:       CapField(init.Carryover),
	}
}

func (c CycleInputs) Goal() string { return c.goal }

func (c CycleInputs) Strategy() string { return c.strategy }

func (c CycleInputs) CommitMessage() string { return c.commitMessage }

func (c CycleInputs) FleetScope() string { return c.fleetScope }

func (c CycleInputs) ChallengeToken() string { return c.challengeToken }

func (c CycleInputs) PreviousVerdict() string { return c.previousVerdict }

func (c CycleInputs) Carryover() string { return c.carryover }

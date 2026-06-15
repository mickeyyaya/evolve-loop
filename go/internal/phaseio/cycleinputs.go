package phaseio

// CycleInputsInit is the construction DTO for NewCycleInputs.
type CycleInputsInit struct {
	Goal            string
	Strategy        string
	CommitMessage   string
	FleetScope      string
	ChallengeToken  string
	PreviousVerdict string
	Carryover       string
}

// CycleInputs is the sealed, getters-only view of the cycle-scoped inputs a
// phase needs (goal/strategy/commit-message/fleet-scope/challenge-token/
// carryover). It replaces the ad-hoc, mutable req.Context["goal"]/["strategy"]/…
// string map: values are set once at construction and exposed read-only, so no
// phase can mutate what a sibling observes (P4/P5). The zero value is valid and
// empty.
type CycleInputs struct {
	goal            string
	strategy        string
	commitMessage   string
	fleetScope      string
	challengeToken  string
	previousVerdict string
	carryover       string
}

// NewCycleInputs builds a sealed CycleInputs from init.
func NewCycleInputs(init CycleInputsInit) CycleInputs {
	return CycleInputs{
		goal:            init.Goal,
		strategy:        init.Strategy,
		commitMessage:   init.CommitMessage,
		fleetScope:      init.FleetScope,
		challengeToken:  init.ChallengeToken,
		previousVerdict: init.PreviousVerdict,
		carryover:       init.Carryover,
	}
}

// Goal returns the cycle goal (formerly req.Context["goal"]).
func (c CycleInputs) Goal() string { return c.goal }

// Strategy returns the cycle strategy (formerly req.Context["strategy"]).
func (c CycleInputs) Strategy() string { return c.strategy }

// CommitMessage returns the ship commit message (formerly req.Context["commit_message"]).
func (c CycleInputs) CommitMessage() string { return c.commitMessage }

// FleetScope returns the fleet partition scope (formerly req.Context["fleet_scope"]).
func (c CycleInputs) FleetScope() string { return c.fleetScope }

// ChallengeToken returns the intent challenge token (formerly the live
// req.Context["challengeToken"] — camelCase key; "challenge_token" is the
// wire-JSON field name, not the Context key).
func (c CycleInputs) ChallengeToken() string { return c.challengeToken }

// PreviousVerdict returns the prior phase's verdict, set for the retro phase
// (formerly req.Context["previous_verdict"]). Empty for non-retro phases.
func (c CycleInputs) PreviousVerdict() string { return c.previousVerdict }

// Carryover returns the carried-over backlog summary the triage phase reads
// (formerly req.Context["carryover_summary"] — note the legacy Context key is
// carryover_summary, not carryover). Empty when there is no carryover.
func (c CycleInputs) Carryover() string { return c.carryover }

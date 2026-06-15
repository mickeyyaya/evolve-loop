package phaseio

import "testing"

func TestCycleInputs_Getters_ReturnConstructedValues(t *testing.T) {
	c := NewCycleInputs(CycleInputsInit{
		Goal:            "reduce dispatch latency",
		Strategy:        "profile-first",
		CommitMessage:   "perf(core): cache HEAD",
		FleetScope:      "core,bridge",
		ChallengeToken:  "tok-123",
		PreviousVerdict: "FAIL",
		Carryover:       "carried: tighten the digest fallback",
	})

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"Goal", c.Goal(), "reduce dispatch latency"},
		{"Strategy", c.Strategy(), "profile-first"},
		{"CommitMessage", c.CommitMessage(), "perf(core): cache HEAD"},
		{"FleetScope", c.FleetScope(), "core,bridge"},
		{"ChallengeToken", c.ChallengeToken(), "tok-123"},
		{"PreviousVerdict", c.PreviousVerdict(), "FAIL"},
		{"Carryover", c.Carryover(), "carried: tighten the digest fallback"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s() = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

func TestCycleInputs_Zero_AllGettersEmpty(t *testing.T) {
	var c CycleInputs // zero value must be safe and empty
	if c.Goal() != "" || c.Strategy() != "" || c.CommitMessage() != "" || c.FleetScope() != "" || c.ChallengeToken() != "" || c.PreviousVerdict() != "" || c.Carryover() != "" {
		t.Fatalf("zero CycleInputs not empty: %+v", c)
	}
}

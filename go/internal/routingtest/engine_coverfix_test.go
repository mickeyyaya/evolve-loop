package routingtest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

type errorCollector struct{ messages []string }

func (c *errorCollector) Errorf(format string, args ...any) {
	c.messages = append(c.messages, fmt.Sprintf(format, args...))
}

func TestCheckPureExpectations_ReportsEachMismatchedExpectation(t *testing.T) {
	t.Parallel()
	got := router.RouterDecision{
		NextPhase:     "build",
		InsertPhases:  []string{"tester"},
		SkipPhases:    []string{"tdd"},
		Clamps:        []router.Clamp{{Rule: "mandatory"}},
		Reason:        "static",
		Justification: "because of the plan",
	}
	cases := []struct {
		name   string
		expect ExpectSpec
		want   string
	}{
		{"next phase", ExpectSpec{NextPhase: "audit"}, `NextPhase="build", want "audit"`},
		{"inserts", ExpectSpec{Inserts: []string{"retro"}}, "InsertPhases=[tester], want [retro]"},
		{"skips", ExpectSpec{Skips: []string{"scout"}}, "SkipPhases=[tdd] must contain [scout]"},
		{"clamps", ExpectSpec{Clamps: []string{"no-such-rule"}}, `missing clamp "no-such-rule"`},
		{"reason", ExpectSpec{Reason: "dynamic"}, `Reason="static", want "dynamic"`},
		{"justification", ExpectSpec{Justification: "absent words"}, `want substring "absent words"`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := &errorCollector{}
			checkPureExpectations(c, ScenarioSpec{Expect: tc.expect}, got)
			if len(c.messages) != 1 || !strings.Contains(c.messages[0], tc.want) {
				t.Fatalf("messages = %q, want exactly one containing %q", c.messages, tc.want)
			}
		})
	}
}

func TestCheckPureExpectations_MatchingDecisionReportsNothing(t *testing.T) {
	t.Parallel()
	got := router.RouterDecision{
		NextPhase:     "build",
		InsertPhases:  []string{"tester"},
		SkipPhases:    []string{"tdd", "scout"},
		Clamps:        []router.Clamp{{Rule: "mandatory"}},
		Reason:        "static",
		Justification: "because of the plan",
	}
	spec := ScenarioSpec{Expect: ExpectSpec{
		NextPhase:     "build",
		Inserts:       []string{"tester"},
		Skips:         []string{"tdd"},
		Clamps:        []string{"mandatory"},
		Reason:        "static",
		Justification: "the plan",
	}}
	c := &errorCollector{}
	checkPureExpectations(c, spec, got)
	if len(c.messages) != 0 {
		t.Fatalf("matching decision reported %q", c.messages)
	}
}

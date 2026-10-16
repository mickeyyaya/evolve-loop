package scout

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
)

func TestScout_ComposePrompt_RendersFleetScope(t *testing.T) {
	req := core.PhaseRequest{Context: map[string]string{"fleet_scope": "todo-lane-a,todo-extra"}}
	out := hooks{}.ComposePrompt("BODY", req)
	for _, id := range []string{"todo-lane-a", "todo-extra"} {
		if !regexp.MustCompile(`(?m)^.*fleet_scope.*` + regexp.QuoteMeta(id)).MatchString(out) {
			t.Errorf("scout prompt has no fleet_scope directive naming %q:\n%s", id, out)
		}
	}
}

func TestScout_ComposePrompt_TwoLanesSeeOnlyOwnScope(t *testing.T) {
	lanes := []struct{ own, other string }{
		{own: "todo-lane-a", other: "todo-lane-b"},
		{own: "todo-lane-b", other: "todo-lane-a"},
	}
	for _, lane := range lanes {
		req := core.PhaseRequest{Context: map[string]string{"fleet_scope": lane.own}}
		out := hooks{}.ComposePrompt("BODY", req)
		if !strings.Contains(out, lane.own) {
			t.Errorf("lane %q: own scope missing from scout prompt:\n%s", lane.own, out)
		}
		if strings.Contains(out, lane.other) {
			t.Errorf("lane %q: FOREIGN lane id %q leaked into scout prompt:\n%s", lane.own, lane.other, out)
		}
	}
}

func TestScout_ComposePrompt_FleetScopeFromTypedEnvelope(t *testing.T) {
	req := core.PhaseRequest{
		Input: phaseio.NewPhaseInput(phaseio.PhaseInputInit{
			Phase:       "scout",
			CycleInputs: phaseio.NewCycleInputs(phaseio.CycleInputsInit{FleetScope: "todo-typed-lane"}),
		}),
	}
	out := hooks{}.ComposePrompt("BODY", req)
	if !regexp.MustCompile(`(?m)^.*fleet_scope.*todo-typed-lane`).MatchString(out) {
		t.Errorf("scout prompt has no fleet_scope directive from the typed envelope:\n%s", out)
	}
}

func TestScout_ComposePrompt_LaneScoped_InstructsGoalHashEchoInDecisionTrace(t *testing.T) {
	req := core.PhaseRequest{
		GoalHash: "goal-hash-776",
		Context:  map[string]string{"fleet_scope": "todo-lane-a"},
	}
	out := hooks{}.ComposePrompt("BODY", req)
	echo := regexp.MustCompile(`(?im)^.*goal_hash.*Decision Trace.*$|^.*Decision Trace.*goal_hash.*$`)
	if !echo.MatchString(out) {
		t.Errorf("lane-scoped scout prompt never instructs a Decision Trace goal_hash echo:\n%s", out)
	}
}

func TestScout_ComposePrompt_NoFleetScope_NoScopeLine(t *testing.T) {
	out := hooks{}.ComposePrompt("BODY", core.PhaseRequest{Context: map[string]string{}})
	if strings.Contains(out, "fleet_scope") {
		t.Errorf("unscoped scout prompt must not carry a fleet_scope line:\n%s", out)
	}
}

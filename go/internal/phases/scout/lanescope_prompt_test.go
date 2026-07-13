package scout

// Cycle-776 RED contract — fleet-lane-provisioning-split (residual slice).
// Cycle-766 pinned lane identity into Context["fleet_scope"] for every phase,
// but only triage RENDERS it into the LLM prompt (triage.go). Cycle-776's own
// run is the live proof of the gap: scout-prompt.txt carried no lane scope, so
// scout scouted three out-of-scope tasks and triage had to defer all of them.
//
// Contract encoded here (Builder implements, must NOT modify these tests):
//
//  1. RENDER: when the lane scope is present (Context["fleet_scope"] or the
//     typed envelope at enforce), scout's composed prompt carries a
//     fleet_scope directive line naming the assigned todo ids — scout must
//     scout ONLY within them.
//  2. ISOLATION (negative): each lane's prompt names only its OWN ids; the
//     other lane's ids must not appear (cycle-640 cross-lane drift).
//  3. GATE TEETH: a lane-scoped scout prompt instructs the Decision Trace to
//     echo the pinned goal_hash — without the echo the scout→triage
//     coherence gate (lanescope.go) fails open forever (cycle-776's own
//     scout omitted it and the gate never fired).
//  4. EDGE: no lane scope ⇒ no fleet_scope line (sequential cycles
//     byte-identical; guards against over-rendering).

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
)

// RENDER: a Context-carried lane scope appears in the composed prompt as a
// fleet_scope directive line naming every assigned todo id.
func TestScout_ComposePrompt_RendersFleetScope(t *testing.T) {
	req := core.PhaseRequest{Context: map[string]string{"fleet_scope": "todo-lane-a,todo-extra"}}
	out := hooks{}.ComposePrompt("BODY", req)
	for _, id := range []string{"todo-lane-a", "todo-extra"} {
		if !regexp.MustCompile(`(?m)^.*fleet_scope.*` + regexp.QuoteMeta(id)).MatchString(out) {
			t.Errorf("scout prompt has no fleet_scope directive naming %q:\n%s", id, out)
		}
	}
}

// ISOLATION (negative): two lanes with distinct scopes each render ONLY their
// own ids — the other lane's id must be absent from the composed prompt.
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

// RENDER/typed: at enforce the scope comes from the typed envelope with NO
// Context map at all (mirror of triage.go's dual read — proves the typed
// source is genuinely consulted).
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

// GATE TEETH: a lane-scoped prompt instructs scout to echo goal_hash in its
// Decision Trace (same line mentions both), so the lanescope.go coherence
// gate can actually fire on a mismatch instead of failing open forever.
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

// EDGE: no lane scope anywhere ⇒ no fleet_scope line at all (sequential /
// single-lane cycles must stay byte-identical).
func TestScout_ComposePrompt_NoFleetScope_NoScopeLine(t *testing.T) {
	out := hooks{}.ComposePrompt("BODY", core.PhaseRequest{Context: map[string]string{}})
	if strings.Contains(out, "fleet_scope") {
		t.Errorf("unscoped scout prompt must not carry a fleet_scope line:\n%s", out)
	}
}

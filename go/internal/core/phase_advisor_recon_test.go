package core

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// TestComposePlanPrompt_RendersReconWhenEnabled pins WS2-S0b's rollout gate
// (ADR-0052): the deterministic pre-plan recon section appears in the plan prompt
// ONLY when EVOLVE_ROUTER_RECON_DIGEST is on (cfg.ReconDigest). Off (default) is
// byte-identical to pre-slice — the whole point of the dial. The gather fails
// open on a non-existent ProjectRoot, so the goal-keyword/carryover facts still
// drive a non-zero digest here without a real git repo.
func TestComposePlanPrompt_RendersReconWhenEnabled(t *testing.T) {
	t.Parallel()
	in := baseRouteInput()
	in.GoalText = "fix a security bug in the auth flow"
	in.CarryoverTodos = []router.CarryoverTodo{{ID: "t1", Action: "follow up"}}
	p := NewPhaseAdvisor(nil, WithPersona("PERSONA"))

	// Off (default): no recon section, byte-identical path.
	in.Cfg.ReconDigest = false
	off := p.composePlanPrompt(in, "routing-plan.json")
	if strings.Contains(off, "Pre-plan recon") {
		t.Errorf("recon must NOT render when EVOLVE_ROUTER_RECON_DIGEST=off; prompt:\n%s", off)
	}

	// On: the recon section renders with the gathered facts.
	in.Cfg.ReconDigest = true
	on := p.composePlanPrompt(in, "routing-plan.json")
	if !strings.Contains(on, "Pre-plan recon (deterministic)") {
		t.Errorf("recon must render when enabled; prompt:\n%s", on)
	}
	if !strings.Contains(on, "security") {
		t.Errorf("recon should surface goal keyword hits (security/bug/fix); prompt:\n%s", on)
	}
	if !strings.Contains(on, "carryover_count: 1") {
		t.Errorf("recon should surface carryover_count from RouteInput; prompt:\n%s", on)
	}
}

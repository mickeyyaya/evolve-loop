package core

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

// TestMintConfigsFrom_RejectsAdvisorRoleMint pins the WS1-S2 recursion guard
// (ADR-0052 D1, primary defense): the advisor proposes phases for the executed
// spine, NEVER another router/advisor — a brain minting a brain is the one
// recursion the layering forbids. A mint whose name is a reserved control-plane
// identity (router/evolve-router/advisor/failure-advisor, case-insensitive) is
// dropped from the registered set with an observable reason; legitimate mints
// pass through untouched.
func TestMintConfigsFrom_RejectsAdvisorRoleMint(t *testing.T) {
	t.Parallel()
	entries := []router.PhasePlanEntry{
		{Phase: "router", Run: true, Mint: &router.MintSpec{Prompt: "be a router"}},
		{Phase: "evolve-router", Run: true, Mint: &router.MintSpec{Prompt: "x"}},
		{Phase: "Failure-Advisor", Run: true, Mint: &router.MintSpec{Prompt: "x"}}, // case-insensitive
		{Phase: "new-helper", Run: true, Mint: &router.MintSpec{Prompt: "legit"}},
	}
	got := mintConfigsFrom(entries)
	if len(got) != 1 || got[0].Name != "new-helper" {
		t.Fatalf("recursion guard failed: minted configs = %+v, want only new-helper", got)
	}
	// The drop is observable (a recorded reason), and a legitimate name is allowed.
	if reservedAdvisorMintReason("router") == "" {
		t.Error("reservedAdvisorMintReason(router) = empty, want a non-empty reason")
	}
	if reservedAdvisorMintReason("EVOLVE-ROUTER") == "" {
		t.Error("guard must be case-insensitive")
	}
	if reservedAdvisorMintReason("new-helper") != "" {
		t.Error("reservedAdvisorMintReason(new-helper) must be empty (allowed)")
	}
}

// TestAdvisorLaunch_DepthGuard pins the WS1-S2 secondary recursion guard.
// EVOLVE_ADVISOR_DEPTH was retired in cycle-10 (flag-reduction campaign);
// AdvisorDepthExceeded is now dormant (always false). The primary guard
// (reservedAdvisorNames denylist, tested above) remains the live defense.
// This test verifies WithDepthCheck still compiles and that the dormant guard
// never blocks the advisor path.
func TestAdvisorLaunch_DepthGuard(t *testing.T) {
	t.Parallel()
	plan := `[{"phase":"scout","run":true,"justification":"x"}]`

	// Depth guard is dormant: any env value (including "1") must NOT error.
	fb := &fakeBridge{stdout: plan}
	in := baseRouteInput()
	in.Env = map[string]string{}
	if _, err := NewPhaseAdvisor(fb, WithDepthCheck(AdvisorDepthExceeded)).Plan(in); err != nil {
		t.Fatalf("dormant depth guard must not block advisor: %v", err)
	}
	if fb.calls != 1 {
		t.Errorf("bridge must be called once; got %d", fb.calls)
	}
}

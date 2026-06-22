package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
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

// TestAdvisorLaunch_DepthGuard pins the WS1-S2 env-stamp defense-in-depth: an
// advisor wired with WithDepthCheck(AdvisorDepthExceeded) errors BEFORE dispatch
// when EVOLVE_ADVISOR_DEPTH≥1, so even if the primary denylist were ever bypassed
// a nested advisor cannot run. depth 0 / unset is byte-identical to today.
func TestAdvisorLaunch_DepthGuard(t *testing.T) {
	t.Parallel()
	plan := `[{"phase":"scout","run":true,"justification":"x"}]`

	fb := &fakeBridge{stdout: plan}
	in := baseRouteInput()
	in.Env = map[string]string{"EVOLVE_ADVISOR_DEPTH": "1"}
	if _, err := NewPhaseAdvisor(fb, WithDepthCheck(AdvisorDepthExceeded)).Plan(in); err == nil {
		t.Fatal("Plan with EVOLVE_ADVISOR_DEPTH=1 must error (recursion guard → static)")
	}
	if fb.calls != 0 {
		t.Errorf("guard must short-circuit BEFORE bridge launch; bridge calls=%d, want 0", fb.calls)
	}

	// Unset / 0 must not trip the guard (no behavior change for the normal path).
	ok := &fakeBridge{stdout: plan}
	in0 := baseRouteInput()
	in0.Env = map[string]string{"EVOLVE_ADVISOR_DEPTH": "0"}
	if _, err := NewPhaseAdvisor(ok, WithDepthCheck(AdvisorDepthExceeded)).Plan(in0); err != nil {
		t.Fatalf("depth 0 must not trip the guard: %v", err)
	}
}

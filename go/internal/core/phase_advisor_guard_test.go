package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

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

func TestAdvisorLaunch_DepthGuard(t *testing.T) {
	t.Parallel()
	plan := `[{"phase":"scout","run":true,"justification":"x"}]`

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

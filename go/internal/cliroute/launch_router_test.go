package cliroute_test

import (
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestRefuseLaunchRouter_ADeclaredTableNeedsTheRootsRouter(t *testing.T) {
	declared := routingPolicy(policy.CLIRouting{CLIs: []string{"claude"}, Default: []string{"claude"}})

	err := cliroute.RefuseLaunchRouter(declared)

	if !errors.Is(err, cliroute.ErrNoRootRouter) || !errors.Is(err, cliroute.ErrRefused) {
		t.Fatalf("err = %v, want a refusal naming the missing root router", err)
	}
}

func TestRefuseLaunchRouter_NoTableKeepsThePerLaunchLegacyRouter(t *testing.T) {
	if err := cliroute.RefuseLaunchRouter(policy.Policy{Pins: map[string]policy.Pin{"scout": {CLI: "claude"}}}); err != nil {
		t.Fatalf("a legacy policy routes per launch as before: %v", err)
	}
}

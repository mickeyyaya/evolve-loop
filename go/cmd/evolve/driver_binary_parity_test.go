package main

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func TestDriverBinaryParity_EveryRegisteredBridgeDriverIsKnownToRoutingOnItsManifestBinary(t *testing.T) {
	registered := bridge.DriverNames()
	if len(registered) == 0 {
		t.Fatal("bridge registers no driver: the parity check would be vacuous")
	}
	for _, driver := range registered {
		m, err := bridge.LoadManifest(driver)
		if err != nil {
			t.Errorf("registered driver %s has no loadable manifest: %v", driver, err)
			continue
		}
		if !llmroute.KnownDriver(driver) {
			t.Errorf("bridge registers %s but llmroute does not know it: routing would never place it in a chain", driver)
			continue
		}
		if got := llmroute.Binary(driver); got != m.Binary {
			t.Errorf("llmroute.Binary(%s) = %q, manifest binary = %q: Probe would look up a binary the driver does not run", driver, got, m.Binary)
		}
	}
}

func TestDriverBinaryParity_EveryRoutingDriverIsARegisteredBridgeDriver(t *testing.T) {
	registered := bridge.DriverNames()
	for _, driver := range llmroute.Drivers() {
		if !slices.Contains(registered, driver) {
			t.Errorf("llmroute knows %s but bridge registers no such driver: a chain naming it fails at launch", driver)
		}
	}
}

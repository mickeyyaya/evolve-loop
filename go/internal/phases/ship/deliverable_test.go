package ship

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestIntegrationCodesAreRegisteredUnderTheShipModule(t *testing.T) {
	t.Parallel()
	for _, c := range []signalcenter.Code{
		"SHIP_INTEGRATION_DELIVERED", "SHIP_INTEGRATION_COMPOSED", "SHIP_INTEGRATION_EJECTED",
		"SHIP_INTEGRATION_FLUSHED", "SHIP_INTEGRATION_MERGED", "SHIP_INTEGRATION_REVERTED",
	} {
		m, ok := signalcenter.IsRegistered(c)
		if !ok || m != signalcenter.ModuleShip {
			t.Errorf("%s: module=%q registered=%v, want ship and registered", c, m, ok)
		}
	}
}

func TestDeliveredKindIsKnown(t *testing.T) {
	t.Parallel()
	if !signalcenter.KindShipDelivered.Known() || signalcenter.KindShipDelivered != "ship.delivered" {
		t.Fatalf("kind %q is not a known kind", signalcenter.KindShipDelivered)
	}
}

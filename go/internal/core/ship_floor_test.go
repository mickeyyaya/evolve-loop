package core

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

func TestResolvedShipFloor(t *testing.T) {
	t.Parallel()
	t.Run("unset falls back to the router default", func(t *testing.T) {
		o := &Orchestrator{}
		got := o.resolvedShipFloor()
		want := router.DefaultShipFloor()
		if len(got) != len(want) {
			t.Fatalf("default floor = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("floor[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("configured floor is used verbatim (audit-only posture)", func(t *testing.T) {
		o := &Orchestrator{}
		WithShipFloor([]string{"audit"})(o)
		got := o.resolvedShipFloor()
		if len(got) != 1 || got[0] != "audit" {
			t.Errorf("floor = %v, want [audit]", got)
		}
	})

	t.Run("WithShipFloor ignores empty (keeps default)", func(t *testing.T) {
		o := &Orchestrator{}
		WithShipFloor(nil)(o)
		if len(o.shipFloor) != 0 {
			t.Errorf("empty floor must be ignored, got %v", o.shipFloor)
		}
		if len(o.resolvedShipFloor()) != len(router.DefaultShipFloor()) {
			t.Error("resolvedShipFloor should still return the default after an ignored empty set")
		}
	})
}

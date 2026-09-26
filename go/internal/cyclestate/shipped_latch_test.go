package cyclestate

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCycleState_ShippedRoundTripsAndIsOmittedWhenFalse(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(CycleState{CycleID: 7, Shipped: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"shipped":true`) {
		t.Errorf("Shipped=true not persisted under `shipped`: %s", raw)
	}
	var back CycleState
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !back.Shipped {
		t.Errorf("Shipped=true did not survive the round trip: %+v", back)
	}
	raw, err = json.Marshal(CycleState{CycleID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "shipped") {
		t.Errorf("Shipped=false must be omitted (legacy checkpoints stay byte-identical): %s", raw)
	}
}

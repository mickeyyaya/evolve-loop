package phasespec

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRecoveryMap_JSONRoundTrip pins the PA-DDK DDK-6 recovery descriptor's wire
// contract and omitempty behavior.
func TestRecoveryMap_JSONRoundTrip(t *testing.T) {
	var r RecoveryMap = RecoveryMap{Targets: map[string]string{"PASS": "ship"}}
	raw, err := json.Marshal(PhaseSpec{Name: "retrospective", Recovery: &r})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"recovery":{"targets":{"PASS":"ship"}}`) {
		t.Errorf("recovery must serialize under recovery.targets:\n%s", raw)
	}

	var rt PhaseSpec
	if err := json.Unmarshal(raw, &rt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rt.Recovery == nil || rt.Recovery.Targets["PASS"] != "ship" {
		t.Errorf("recovery round-trip lost data: %+v", rt.Recovery)
	}

	bare, _ := json.Marshal(PhaseSpec{Name: "scout"})
	if strings.Contains(string(bare), `"recovery":`) {
		t.Errorf("unset recovery must be omitted:\n%s", bare)
	}
}

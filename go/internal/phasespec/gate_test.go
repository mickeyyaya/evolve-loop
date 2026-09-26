package phasespec

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestArtifactGate_JSONRoundTrip(t *testing.T) {
	var g ArtifactGate = ArtifactGate{RequiresPresent: true, VerdictIn: []string{"PASS", "WARN"}}
	raw, err := json.Marshal(PhaseSpec{Name: "audit", Gate: &g})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"requires_present":true`) || !strings.Contains(string(raw), `"verdict_in":["PASS","WARN"]`) {
		t.Errorf("gate must serialize under requires_present/verdict_in:\n%s", raw)
	}

	var rt PhaseSpec
	if err := json.Unmarshal(raw, &rt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rt.Gate == nil || !rt.Gate.RequiresPresent || len(rt.Gate.VerdictIn) != 2 {
		t.Errorf("gate round-trip lost data: %+v", rt.Gate)
	}

	// Matching `"gate":` with the colon keeps the distinct "gates" key from matching.
	bare, _ := json.Marshal(PhaseSpec{Name: "scout"})
	if strings.Contains(string(bare), `"gate":`) {
		t.Errorf("unset gate must be omitted:\n%s", bare)
	}
}

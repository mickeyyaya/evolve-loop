package policy

import (
	"encoding/json"
	"testing"
	"time"
)

func TestResumeConfig_TheCheckpointAgeBoundDefaultsToEightDaysAndIsConfigurable(t *testing.T) {
	if got := (Policy{}).ResumeConfig().MaxCheckpointAge(); got != 192*time.Hour {
		t.Fatalf("default MaxCheckpointAge = %v, want 192h: a quota pause waits at most for the weekly reset", got)
	}
	var p Policy
	if err := json.Unmarshal([]byte(`{"resume":{"max_checkpoint_age_h":24}}`), &p); err != nil {
		t.Fatal(err)
	}
	if got := p.ResumeConfig().MaxCheckpointAge(); got != 24*time.Hour {
		t.Fatalf("configured MaxCheckpointAge = %v, want 24h", got)
	}
}

package policy

import (
	"encoding/json"
	"strings"
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

func TestResumeConfig_UnmarshalJSONRefusesAnUnknownKeyAndANonPositiveAge(t *testing.T) {
	for raw, want := range map[string]string{
		`{"resume":{"max_checkpoint_age":24}}`:   "resume: json: unknown field",
		`{"resume":{"max_checkpoint_age_h":0}}`:  "resume: max_checkpoint_age_h must be positive, got 0",
		`{"resume":{"max_checkpoint_age_h":-3}}`: "resume: max_checkpoint_age_h must be positive, got -3",
	} {
		var p Policy
		if err := json.Unmarshal([]byte(raw), &p); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Unmarshal(%s) err = %v, want it to contain %q", raw, err, want)
		}
	}
	var p Policy
	if err := json.Unmarshal([]byte(`{"resume":{}}`), &p); err != nil || p.ResumeConfig().MaxCheckpointAge() != 192*time.Hour {
		t.Errorf("an empty block = %+v, %v; want the 192 h default", p.Resume, err)
	}
}

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunFleet_RejectsMissingCount(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := runFleet([]string{"--goal-hash", "abc"}, nil, &out, &errb); rc != 1 {
		t.Fatalf("rc=%d, want 1", rc)
	}
	if !strings.Contains(errb.String(), "count") {
		t.Errorf("stderr=%q, want a --count error", errb.String())
	}
}

func TestRunFleet_RejectsMissingGoalHash(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := runFleet([]string{"--count", "3"}, nil, &out, &errb); rc != 1 {
		t.Fatalf("rc=%d, want 1", rc)
	}
	if !strings.Contains(errb.String(), "goal-hash") {
		t.Errorf("stderr=%q, want a --goal-hash error", errb.String())
	}
}

// TestDispatch_FleetRegistered: `evolve fleet` must route to runFleet, not the
// dispatch unknown-command path (rc 2). Bad args → rc 1 from runFleet's validation.
func TestDispatch_FleetRegistered(t *testing.T) {
	var out, errb bytes.Buffer
	rc := dispatch([]string{"fleet"}, nil, &out, &errb)
	if rc != 1 {
		t.Fatalf("dispatch fleet rc=%d, want 1 (runFleet validation), not 2 (unknown command)", rc)
	}
	if strings.Contains(errb.String(), "unknown command") {
		t.Errorf("fleet not registered in the command table: %q", errb.String())
	}
}

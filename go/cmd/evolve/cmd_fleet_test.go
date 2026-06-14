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

// TestLoadPlanSpecs_AssignsDisjointScopesWithGoalHash: --plan partitions the
// backlog into disjoint-scoped specs, each stamped with the goal hash; a todo
// bridging two cycles defers.
func TestLoadPlanSpecs_AssignsDisjointScopesWithGoalHash(t *testing.T) {
	planJSON := []byte(`[
		{"id":"t1","files":["a.go"]},
		{"id":"t2","files":["b.go"]},
		{"id":"t3","files":["a.go","b.go"]}
	]`)
	specs, deferred, err := loadPlanSpecs(planJSON, "gh123", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 2 {
		t.Fatalf("specs=%d, want 2", len(specs))
	}
	for i, s := range specs {
		if s.GoalHash != "gh123" {
			t.Errorf("spec %d GoalHash=%q, want gh123", i, s.GoalHash)
		}
		if s.Env["EVOLVE_FLEET_SCOPE"] == "" {
			t.Errorf("spec %d missing EVOLVE_FLEET_SCOPE", i)
		}
	}
	if len(deferred) != 1 || deferred[0].ID != "t3" {
		t.Errorf("deferred=%v, want [t3]", deferred)
	}
}

func TestLoadPlanSpecs_BadJSON(t *testing.T) {
	if _, _, err := loadPlanSpecs([]byte("{not json"), "gh", 2); err == nil {
		t.Error("want error on malformed --plan JSON")
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

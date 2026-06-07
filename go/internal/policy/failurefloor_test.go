package policy

// failurefloor_test.go — failure floor Phase 4a: .evolve/policy.json is
// the ONE user surface for failure-learning policy (replacing the
// scattered env-flag/registry/config/router enable chain). The
// deterministic floor itself is NON-configurable — failure_floor only
// tunes the LLM-learning richness and the audit-FAIL route.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFailurePolicy_DefaultsAlwaysLearn(t *testing.T) {
	t.Parallel()
	// Zero policy (no failure_floor key) → full learning, retrospective.
	alwaysLearn, route := Policy{}.FailurePolicy()
	if !alwaysLearn {
		t.Error("always_learn must default to true")
	}
	if route != "retrospective" {
		t.Errorf("audit_fail_routes_to default = %q, want retrospective", route)
	}
}

func TestFailurePolicy_AuditFailRouteConfigurable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		ff        *FailureFloor
		wantLearn bool
		wantRoute string
	}{
		{"explicit memo route", &FailureFloor{AuditFailRoutesTo: "memo"}, true, "memo"},
		{"explicit retrospective", &FailureFloor{AuditFailRoutesTo: "retrospective"}, true, "retrospective"},
		{"always_learn false tunes richness only", &FailureFloor{AlwaysLearn: boolPtr(false)}, false, "retrospective"},
		{"unknown route value falls back to default", &FailureFloor{AuditFailRoutesTo: "skip-learning"}, true, "retrospective"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			learn, route := Policy{FailureFloor: tc.ff}.FailurePolicy()
			if learn != tc.wantLearn || route != tc.wantRoute {
				t.Errorf("FailurePolicy() = (%v, %q), want (%v, %q)", learn, route, tc.wantLearn, tc.wantRoute)
			}
		})
	}
}

func TestLoad_ParsesFailureFloor(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"failure_floor":{"always_learn":false,"audit_fail_routes_to":"memo"}}`), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	pol, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	learn, route := pol.FailurePolicy()
	if learn || route != "memo" {
		t.Errorf("loaded FailurePolicy() = (%v, %q), want (false, memo)", learn, route)
	}
}

func boolPtr(b bool) *bool { return &b }

package policy_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// TestDocsFloorConfigDefaults pins the compiled default and every override
// path: the ADR-0077 floor is armed without a policy block, and the operator
// dial in .evolve/policy.json reaches it (config-injected, no flag).
func TestDocsFloorConfigDefaults(t *testing.T) {
	cases := []struct {
		name string
		p    policy.Policy
		want policy.DocsFloorPolicy
	}{
		{
			name: "absent block ⇒ compiled default enforce",
			p:    policy.Policy{},
			want: policy.DocsFloorPolicy{Stage: "enforce"},
		},
		{
			name: "empty block ⇒ compiled default enforce",
			p:    policy.Policy{DocsFloor: &policy.DocsFloorPolicy{}},
			want: policy.DocsFloorPolicy{Stage: "enforce"},
		},
		{
			name: "explicit off wins",
			p:    policy.Policy{DocsFloor: &policy.DocsFloorPolicy{Stage: "off"}},
			want: policy.DocsFloorPolicy{Stage: "off"},
		},
		{
			name: "explicit shadow wins",
			p:    policy.Policy{DocsFloor: &policy.DocsFloorPolicy{Stage: "shadow"}},
			want: policy.DocsFloorPolicy{Stage: "shadow"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.p.DocsFloorConfig(); got != tc.want {
				t.Errorf("DocsFloorConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestDocsFloorConfigRoundTripsThroughLoad proves the JSON key is the one
// operators write: a real policy.json read must reach DocsFloorConfig.
func TestDocsFloorConfigRoundTripsThroughLoad(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name, body, want string
	}{
		{"bare policy keeps the compiled default", `{}`, "enforce"},
		{"docs_floor.stage override reaches the gate", `{"docs_floor":{"stage":"off"}}`, "off"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name+".json")
			if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
				t.Fatalf("write policy: %v", err)
			}
			p, err := policy.Load(path)
			if err != nil {
				t.Fatalf("policy.Load: %v", err)
			}
			if got := p.DocsFloorConfig().Stage; got != tc.want {
				t.Errorf("stage = %q, want %q", got, tc.want)
			}
		})
	}
}

package policy_test

// SandboxPolicy — the OS-sandbox config block. NestedFallback gates the
// verified-fallback write-canary (off/shadow/enforce); it defaults "off" so the
// canary is opt-in and a fresh policy.json never runs it.

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestSandboxConfig_Resolution(t *testing.T) {
	cases := []struct {
		name string
		pol  policy.Policy
		want policy.SandboxPolicy
	}{
		{
			"absent-defaults-off",
			policy.Policy{},
			policy.SandboxPolicy{NestedFallback: "off"},
		},
		{
			"empty-block-defaults-off",
			policy.Policy{Sandbox: &policy.SandboxPolicy{}},
			policy.SandboxPolicy{NestedFallback: "off"},
		},
		{
			"shadow-set",
			policy.Policy{Sandbox: &policy.SandboxPolicy{NestedFallback: "shadow"}},
			policy.SandboxPolicy{NestedFallback: "shadow"},
		},
		{
			"enforce-set",
			policy.Policy{Sandbox: &policy.SandboxPolicy{NestedFallback: "enforce"}},
			policy.SandboxPolicy{NestedFallback: "enforce"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.pol.SandboxConfig()
			if got != tc.want {
				t.Errorf("SandboxConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestLoad_SandboxBlock(t *testing.T) {
	cases := []struct {
		name string
		json string
		want policy.SandboxPolicy
	}{
		{
			"absent-block-defaults-off",
			`{}`,
			policy.SandboxPolicy{NestedFallback: "off"},
		},
		{
			"off-explicit",
			`{"sandbox":{"nested_fallback":"off"}}`,
			policy.SandboxPolicy{NestedFallback: "off"},
		},
		{
			"shadow-set",
			`{"sandbox":{"nested_fallback":"shadow"}}`,
			policy.SandboxPolicy{NestedFallback: "shadow"},
		},
		{
			"enforce-set",
			`{"sandbox":{"nested_fallback":"enforce"}}`,
			policy.SandboxPolicy{NestedFallback: "enforce"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pol, err := policy.Load(writeTempPolicy(t, tc.json))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := pol.SandboxConfig(); got != tc.want {
				t.Errorf("after Load, SandboxConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

package policy_test

import (
	"encoding/json"
	"strings"
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

func TestSandboxPolicy_SandboxModeIsAutoOrOnAndNeverOff(t *testing.T) {
	for mode, want := range map[string]string{"": "auto", "auto": "auto", "on": "on"} {
		pol, err := policy.Parse([]byte(`{"sandbox":{"mode":"` + mode + `"}}`))
		if err != nil {
			t.Fatalf("Parse(%q): %v", mode, err)
		}
		if got := pol.SandboxConfig().SandboxMode(); got != want {
			t.Errorf("sandbox.mode %q gives %q, want %q", mode, got, want)
		}
	}
	for _, mode := range []string{"off", "bogus"} {
		_, err := policy.Parse([]byte(`{"sandbox":{"mode":"` + mode + `"}}`))
		if err == nil || !strings.Contains(err.Error(), "--sandbox-off=<reason>") {
			t.Errorf("Parse(sandbox.mode %q) err = %v, want a refusal that names the audited --sandbox-off parameter", mode, err)
		}
	}
	var block policy.SandboxPolicy
	if err := json.Unmarshal([]byte(`{"nested":"x"}`), &block); err == nil || !strings.Contains(err.Error(), "sandbox:") {
		t.Errorf("an unknown key of the sandbox block: err = %v, want a sandbox refusal", err)
	}
	if got := (policy.SandboxPolicy{Mode: "off"}).SandboxMode(); got != "auto" {
		t.Errorf("SandboxMode of a constructed off = %q, want auto: off is the audited --sandbox-off parameter, never a policy value", got)
	}
}

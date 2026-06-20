package policy_test

// RouterPolicy / SwarmPolicy / DispatchConfig — the typed parameters that
// replaced the EVOLVE_ROUTER_* (advisor-maximization), EVOLVE_SWARM_*, and
// EVOLVE_DISPATCH_* env reads (flag-reduction v20). Each accessor encodes the
// non-obvious default rules: RouterConfig RouterReplan→"shadow", ReplanDepth→1
// (override only when >0); SwarmConfig Stage→"shadow", PortBase passes through;
// DispatchConfig Policy→"verify", RepeatThreshold→5 (override only when >0).
// Black-box: drives only the exported accessors + explicit inputs, zero env.

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestRouterConfig_Resolution(t *testing.T) {
	defaults := policy.RouterPolicy{RouterReplan: "shadow", ReplanDepth: 1}
	cases := []struct {
		name string
		pol  policy.Policy
		want policy.RouterPolicy
	}{
		{"absent-defaults", policy.Policy{}, defaults},
		{"empty-block-defaults", policy.Policy{Router: &policy.RouterPolicy{}}, defaults},
		{"replan-depth-zero-falls-to-default", policy.Policy{Router: &policy.RouterPolicy{ReplanDepth: 0}}, defaults},
		{"replan-depth-negative-falls-to-default", policy.Policy{Router: &policy.RouterPolicy{ReplanDepth: -2}}, defaults},
		{"replan-depth-two-override", policy.Policy{Router: &policy.RouterPolicy{ReplanDepth: 2}}, policy.RouterPolicy{RouterReplan: "shadow", ReplanDepth: 2}},
		{"replan-stage-override", policy.Policy{Router: &policy.RouterPolicy{RouterReplan: "advisory"}}, policy.RouterPolicy{RouterReplan: "advisory", ReplanDepth: 1}},
		{"bools-and-models-passthrough", policy.Policy{Router: &policy.RouterPolicy{
			RoutingJudge: true, ReconDigest: true, PlanModel: "opus", ProposeModel: "haiku", CLI: "claude", Model: "sonnet",
		}}, policy.RouterPolicy{
			RouterReplan: "shadow", ReplanDepth: 1, RoutingJudge: true, ReconDigest: true,
			PlanModel: "opus", ProposeModel: "haiku", CLI: "claude", Model: "sonnet",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.pol.RouterConfig(); got != tc.want {
				t.Errorf("RouterConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestSwarmConfig_Resolution(t *testing.T) {
	defaults := policy.SwarmConfig{Stage: "shadow"}
	cases := []struct {
		name string
		pol  policy.Policy
		want policy.SwarmConfig
	}{
		{"absent-defaults", policy.Policy{}, defaults},
		{"empty-block-defaults", policy.Policy{Swarm: &policy.SwarmPolicy{}}, defaults},
		{"stage-override", policy.Policy{Swarm: &policy.SwarmPolicy{Stage: "enforce"}}, policy.SwarmConfig{Stage: "enforce"}},
		{"port-base-passthrough", policy.Policy{Swarm: &policy.SwarmPolicy{PortBase: 4100}}, policy.SwarmConfig{Stage: "shadow", PortBase: 4100}},
		{"stage-and-port", policy.Policy{Swarm: &policy.SwarmPolicy{Stage: "advisory", PortBase: 5000}}, policy.SwarmConfig{Stage: "advisory", PortBase: 5000}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.pol.SwarmConfig(); got != tc.want {
				t.Errorf("SwarmConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestDispatchConfig_Resolution(t *testing.T) {
	defaults := policy.DispatchConfig{Policy: "verify", RepeatThreshold: 5}
	cases := []struct {
		name string
		pol  policy.Policy
		want policy.DispatchConfig
	}{
		{"absent-defaults", policy.Policy{}, defaults},
		{"empty-block-defaults", policy.Policy{Dispatch: &policy.DispatchConfig{}}, defaults},
		{"threshold-zero-falls-to-default", policy.Policy{Dispatch: &policy.DispatchConfig{RepeatThreshold: 0}}, defaults},
		{"threshold-negative-falls-to-default", policy.Policy{Dispatch: &policy.DispatchConfig{RepeatThreshold: -1}}, defaults},
		{"policy-override", policy.Policy{Dispatch: &policy.DispatchConfig{Policy: "stop"}}, policy.DispatchConfig{Policy: "stop", RepeatThreshold: 5}},
		{"threshold-override", policy.Policy{Dispatch: &policy.DispatchConfig{Policy: "off", RepeatThreshold: 3}}, policy.DispatchConfig{Policy: "off", RepeatThreshold: 3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.pol.DispatchConfig(); got != tc.want {
				t.Errorf("DispatchConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestRetryPolicy_UnmarshalJSON pins the custom unmarshaler that records whether
// retry_backoff_base_s / contract_correction_retries were EXPLICITLY present, so
// an explicit 0 (disable) is distinguishable from absent (use default). Drives
// RetryPolicy.UnmarshalJSON directly.
func TestRetryPolicy_UnmarshalJSON(t *testing.T) {
	t.Run("explicit-zero-disables", func(t *testing.T) {
		pol, err := policy.Load(writeTempPolicy(t, `{"retry":{"retry_backoff_base_s":0,"contract_correction_retries":0}}`))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		rc := pol.RetryConfig()
		if rc.RetryBackoffBaseS != 0 || rc.ContractCorrectionRetries != 0 {
			t.Errorf("explicit zero must survive as 0/0, got backoff=%d correction=%d", rc.RetryBackoffBaseS, rc.ContractCorrectionRetries)
		}
	})

	t.Run("absent-uses-defaults", func(t *testing.T) {
		var rp policy.RetryPolicy
		// Call the custom unmarshaler directly so the symbol is named (apicover)
		// and exercised: only phase_max_attempts present, the two "explicit-zero"
		// sentinel fields absent.
		if err := rp.UnmarshalJSON([]byte(`{"phase_max_attempts":3}`)); err != nil {
			t.Fatalf("UnmarshalJSON: %v", err)
		}
		// retry_backoff_base_s absent ⇒ the resolver applies the default (5), not 0.
		rc := (policy.Policy{Retry: &rp}).RetryConfig()
		if rc.RetryBackoffBaseS != 5 || rc.ContractCorrectionRetries != 2 {
			t.Errorf("absent fields must resolve to defaults backoff=5 correction=2, got backoff=%d correction=%d", rc.RetryBackoffBaseS, rc.ContractCorrectionRetries)
		}
	})
}

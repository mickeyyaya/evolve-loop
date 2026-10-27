package policy_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestIsolationEnvStage_ReadsIsolationI2StageAndDefaultsToEmpty(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		``:                 "",
		`{"isolation":{}}`: "",
		`{"isolation":{"i2":{"stage":"enforce"}}}`: "enforce",
		`{"isolation":{"i2":{"stage":"off"}}}`:     "off",
	}
	for body, want := range cases {
		var p policy.Policy
		if body != "" {
			if err := json.Unmarshal([]byte(body), &p); err != nil {
				t.Fatal(err)
			}
		}
		if got := p.IsolationEnvStage(); got != want {
			t.Errorf("%s: IsolationEnvStage = %q, want %q", body, got, want)
		}
	}
}

func TestIsolationPolicy_TheTypedBlockCarriesTheI2Stage(t *testing.T) {
	t.Parallel()
	p := policy.Policy{Isolation: &policy.IsolationPolicy{I2: policy.IsolationInvariantPolicy{Stage: "shadow"}}}
	if got := p.IsolationEnvStage(); got != "shadow" {
		t.Fatalf("IsolationEnvStage = %q, want shadow", got)
	}
}

func TestIsolationExecStage_ReadsIsolationI3StageApartFromI2(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		``:                 "",
		`{"isolation":{}}`: "",
		`{"isolation":{"i2":{"stage":"enforce"}}}`: "",
		`{"isolation":{"i3":{"stage":"enforce"}}}`: "enforce",
	}
	for body, want := range cases {
		var p policy.Policy
		if body != "" {
			if err := json.Unmarshal([]byte(body), &p); err != nil {
				t.Fatal(err)
			}
		}
		if got := p.IsolationExecStage(); got != want {
			t.Errorf("%s: IsolationExecStage = %q, want %q", body, got, want)
		}
	}
}

func TestIsolationFlowStage_ReadsIsolationI8StageAndDefaultsToEmpty(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		``:                 "",
		`{"isolation":{}}`: "",
		`{"isolation":{"i2":{"stage":"enforce"}}}`: "",
		`{"isolation":{"i8":{"stage":"enforce"}}}`: "enforce",
	}
	for body, want := range cases {
		var p policy.Policy
		if body != "" {
			if err := json.Unmarshal([]byte(body), &p); err != nil {
				t.Fatal(err)
			}
		}
		if got := p.IsolationFlowStage(); got != want {
			t.Errorf("%s: IsolationFlowStage = %q, want %q", body, got, want)
		}
	}
}

func TestIsolationReadStageAndLineageMax_ReadTheI5BlockApartFromI2AndI3(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		stage string
		max   int
	}{
		``: {},
		`{"isolation":{"i3":{"stage":"enforce"}}}`:                 {},
		`{"isolation":{"i5":{"stage":"enforce"},"lineage_max":3}}`: {stage: "enforce", max: 3},
		`{"isolation":{"i5":{"stage":"shadow"}}}`:                  {stage: "shadow"},
	}
	for body, want := range cases {
		var p policy.Policy
		if body != "" {
			if err := json.Unmarshal([]byte(body), &p); err != nil {
				t.Fatal(err)
			}
		}
		if got := p.IsolationReadStage(); got != want.stage {
			t.Errorf("%s: IsolationReadStage = %q, want %q", body, got, want.stage)
		}
		if got := p.IsolationLineageMax(); got != want.max {
			t.Errorf("%s: IsolationLineageMax = %d, want %d", body, got, want.max)
		}
	}
}

func TestIsolationGuaranteeStage_ReadsIsolationI7StageAndDefaultsToEmpty(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		``:                 "",
		`{"isolation":{}}`: "",
		`{"isolation":{"i3":{"stage":"enforce"}}}`: "",
		`{"isolation":{"i7":{"stage":"enforce"}}}`: "enforce",
	}
	for body, want := range cases {
		var p policy.Policy
		if body != "" {
			if err := json.Unmarshal([]byte(body), &p); err != nil {
				t.Fatal(err)
			}
		}
		if got := p.IsolationGuaranteeStage(); got != want {
			t.Errorf("%s: IsolationGuaranteeStage = %q, want %q", body, got, want)
		}
	}
}

func TestPolicyParse_AcceptsTheIsolationI7Stage(t *testing.T) {
	t.Parallel()
	p, err := policy.Parse([]byte(`{"isolation":{"i7":{"stage":"enforce"}}}`))
	if err != nil || p.IsolationGuaranteeStage() != "enforce" {
		t.Fatalf("the strict parse must keep isolation.i7.stage: %v %q", err, p.IsolationGuaranteeStage())
	}
}

func TestIsolationContext_ParsesTheStageTheBudgetsAndTheRequiredLists(t *testing.T) {
	t.Parallel()
	p, err := policy.Parse([]byte(`{"isolation":{"context":{"stage":"shadow","budget":{"l1_tokens":100,"l2_tokens":50},"required":{"build":["scout-report"],"audit":[]}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := p.IsolationContext()
	if c.Stage != "shadow" || c.Budget.L1Tokens != 100 || c.Budget.L2Tokens != 50 {
		t.Fatalf("IsolationContext = %+v", c)
	}
	if got, ok := c.Required["build"]; !ok || len(got) != 1 || got[0] != "scout-report" {
		t.Fatalf("required of build = %v %v", got, ok)
	}
	if got, ok := c.Required["audit"]; !ok || got == nil || len(got) != 0 {
		t.Fatalf("an empty list stays an empty list, not an absent one: %v %v", got, ok)
	}
	if empty := (policy.Policy{}).IsolationContext(); empty.Stage != "" || empty.Required != nil {
		t.Fatalf("a policy with no isolation block has no context settings: %+v", empty)
	}
	typed := policy.Policy{Isolation: &policy.IsolationPolicy{Context: &policy.IsolationContextPolicy{Budget: policy.IsolationContextBudget{L2Tokens: 7}}}}
	if got := typed.IsolationContext().Budget.L2Tokens; got != 7 {
		t.Fatalf("the typed block carries its budget: %d", got)
	}
}

func TestParse_RefusesAContextBlockThatNoReaderCanApply(t *testing.T) {
	t.Parallel()
	for body, want := range map[string]string{
		`{"isolation":{"context":{"stage":"enforce"}}}`:                "context-index-l0-prompt-enforce",
		`{"isolation":{"context":{"stage":"loud"}}}`:                   `stage "loud"`,
		`{"isolation":{"context":{"budget":{"l1_tokens":-1}}}}`:        "l1_tokens",
		`{"isolation":{"context":{"budget":{"l2_tokens":-5}}}}`:        "l2_tokens",
		`{"isolation":{"context":{"budget":{"l3_entries":4}}}}`:        "unknown policy key",
		`{"isolation":{"context":{"required":{"build":["Scout/x"]}}}}`: `kind "Scout/x"`,
	} {
		_, err := policy.Parse([]byte(body))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Parse(%s) = %v, want an error that names %q", body, err, want)
		}
	}
	for _, ok := range []string{`{"isolation":{"context":{}}}`, `{"isolation":{"context":{"stage":"shadow"}}}`, `{"isolation":{}}`} {
		if _, err := policy.Parse([]byte(ok)); err != nil {
			t.Errorf("Parse(%s) = %v, want ok", ok, err)
		}
	}
}

func TestIsolationQueueStage_ReadsIsolationI15StageAndDefaultsToEmpty(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		``:                 "",
		`{"isolation":{}}`: "",
		`{"isolation":{"i7":{"stage":"enforce"}}}`:  "",
		`{"isolation":{"i15":{"stage":"enforce"}}}`: "enforce",
	}
	for body, want := range cases {
		var p policy.Policy
		if body != "" {
			if err := json.Unmarshal([]byte(body), &p); err != nil {
				t.Fatal(err)
			}
		}
		if got := p.IsolationQueueStage(); got != want {
			t.Errorf("%s: IsolationQueueStage = %q, want %q", body, got, want)
		}
	}
}

func TestPolicyParse_AcceptsTheIsolationI15Stage(t *testing.T) {
	t.Parallel()
	p, err := policy.Parse([]byte(`{"isolation":{"i15":{"stage":"enforce"}}}`))
	if err != nil || p.IsolationQueueStage() != "enforce" {
		t.Fatalf("Parse = %v, stage %q; want the i15 stage accepted", err, p.IsolationQueueStage())
	}
}

func TestIsolationDispatchRequests_DefaultsAndOverrides(t *testing.T) {
	t.Parallel()
	defaults := policy.Policy{}.IsolationDispatchRequests()
	if defaults.Stage != policy.DispatchRequestStageShadow || defaults.PerCycle != 3 || defaults.MaxDepth != 1 || len(defaults.Roles) == 0 {
		t.Fatalf("defaults = %+v, want stage shadow, 3 requests for each cycle, depth 1 and a default role list", defaults)
	}
	p, err := policy.Parse([]byte(`{"isolation":{"dispatch_requests":{"stage":"enforce","roles":["fault-localization"],"per_cycle":1,"max_depth":2}}}`))
	if err != nil {
		t.Fatal(err)
	}
	got := p.IsolationDispatchRequests()
	if got.Stage != policy.DispatchRequestStageEnforce || got.PerCycle != 1 || got.MaxDepth != 2 || len(got.Roles) != 1 || got.Roles[0] != "fault-localization" {
		t.Fatalf("overrides = %+v", got)
	}
	got.Roles[0] = "changed"
	if again := p.IsolationDispatchRequests(); again.Roles[0] != "fault-localization" {
		t.Fatalf("the accessor must return a copy of the roles, got %+v", again)
	}
	off := policy.Policy{Isolation: &policy.IsolationPolicy{DispatchRequests: &policy.DispatchRequestsPolicy{Stage: policy.DispatchRequestStageOff}}}
	if off.IsolationDispatchRequests().Stage != policy.DispatchRequestStageOff {
		t.Fatal("the stage off must hold")
	}
}

func TestParse_RefusesADispatchRequestsBlockThatNoReaderCanApply(t *testing.T) {
	t.Parallel()
	for body, want := range map[string]string{
		`{"isolation":{"dispatch_requests":{"stage":"loud"}}}`:         `stage "loud"`,
		`{"isolation":{"dispatch_requests":{"per_cycle":-1}}}`:         "per_cycle",
		`{"isolation":{"dispatch_requests":{"max_depth":-1}}}`:         "max_depth",
		`{"isolation":{"dispatch_requests":{"roles":["../builder"]}}}`: `role "../builder"`,
		`{"isolation":{"dispatch_requests":{"budget":3}}}`:             "unknown policy key",
	} {
		_, err := policy.Parse([]byte(body))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Parse(%s) = %v, want an error that names %q", body, err, want)
		}
	}
}

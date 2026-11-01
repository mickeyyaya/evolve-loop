package policy_test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestContinuationConfig_AnAbsentBlockIsTheDefaultAge(t *testing.T) {
	if got := (policy.Policy{}).ContinuationConfig(); got.MaxAgeCycles != 100 {
		t.Errorf("default max_age_cycles = %d, want 100", got.MaxAgeCycles)
	}
	p, err := loadPolicyText(t, `{"continuation": {}}`)
	if err != nil || p.ContinuationConfig().MaxAgeCycles != 100 {
		t.Errorf("an empty block keeps the default: %+v %v", p.ContinuationConfig(), err)
	}
}

func TestContinuationConfig_ANamedAgeInRangeOverridesTheDefault(t *testing.T) {
	for _, age := range []string{"1", "40", "10000"} {
		p, err := loadPolicyText(t, `{"continuation": {"max_age_cycles": `+age+`}}`)
		if err != nil {
			t.Fatalf("age %s: %v", age, err)
		}
		if got := p.ContinuationConfig().MaxAgeCycles; strconv.Itoa(got) != age {
			t.Errorf("max_age_cycles = %d, want %s", got, age)
		}
	}
}

func TestContinuationConfig_RefusesAnAgeOutOfRangeOrAnUnknownKeyByName(t *testing.T) {
	for body, want := range map[string]string{
		`{"continuation": {"max_age_cycles": 0}}`:     "max_age_cycles",
		`{"continuation": {"max_age_cycles": -3}}`:    "max_age_cycles",
		`{"continuation": {"max_age_cycles": 10001}}`: "max_age_cycles",
		`{"continuation": {"max_age_cycle": 40}}`:     "max_age_cycle",
		`{"continuation": {"max_age_cycles": "40"}}`:  "continuation",
	} {
		if _, err := loadPolicyText(t, body); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want a refusal naming %q", body, err, want)
		}
	}
}

func TestContinuationPolicy_DecodesStrictlyOnItsOwn(t *testing.T) {
	var b policy.ContinuationPolicy
	if err := json.Unmarshal([]byte(`{"max_age_cycles": 7}`), &b); err != nil || b.MaxAgeCycles == nil || *b.MaxAgeCycles != 7 {
		t.Errorf("decode = %+v, %v", b, err)
	}
	if err := json.Unmarshal([]byte(`[]`), &b); err == nil {
		t.Error("a block that is not an object must be refused")
	}
}

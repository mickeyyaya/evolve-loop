package policy_test

import (
	"encoding/json"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestAdvisorSkillRejection_JSONShape(t *testing.T) {
	r := policy.AdvisorSkillRejection{Skill: "does-not-exist", Reason: "not-in-registry"}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal AdvisorSkillRejection: %v", err)
	}
	if got, want := string(b), `{"skill":"does-not-exist","reason":"not-in-registry"}`; got != want {
		t.Errorf("AdvisorSkillRejection JSON = %s, want %s", got, want)
	}
}

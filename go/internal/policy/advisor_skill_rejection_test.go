package policy_test

// advisor_skill_rejection_test.go — cycle-613 advisor-skill-selection.
// AdvisorSkillRejection values are logged to advisor-rejections.json, so its
// wire shape (json tags) is part of the contract the rejection-plumbing
// checklist item depends on. This pins that shape and names the type for
// apicover. Separate from advisor_skill_overlay_test.go (the RED clamp/merge
// contract) so that protected file is not modified.

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

package policy_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestInboxTrailerStage_DefaultsToShadow(t *testing.T) {
	for name, text := range map[string]string{
		"empty policy":  `{}`,
		"empty inbox":   `{"inbox": {}}`,
		"empty curator": `{"inbox": {"curator": {}}}`,
		"empty trailer": `{"inbox": {"curator": {"trailer": {}}}}`,
	} {
		p, err := loadPolicyText(t, text)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := p.InboxTrailerStage(); got != "shadow" {
			t.Errorf("%s: InboxTrailerStage() = %q, want shadow", name, got)
		}
	}
}

func TestInboxTrailerStage_ReadsTheNamedStageIndependentlyOfTheCuratorStage(t *testing.T) {
	p, err := loadPolicyText(t, `{"inbox": {"curator": {"stage": "shadow", "trailer": {"stage": "enforce"}}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.InboxTrailerStage(); got != "enforce" {
		t.Errorf("InboxTrailerStage() = %q, want enforce", got)
	}
	if got := p.InboxCuratorConfig().Stage; got != "shadow" {
		t.Errorf("InboxCuratorConfig().Stage = %q, want shadow: the trailer stage must not move it", got)
	}
}

func TestInboxTrailerStage_RefusesAStageThatIsNotShadowOrEnforce(t *testing.T) {
	_, err := loadPolicyText(t, `{"inbox": {"curator": {"trailer": {"stage": "warn"}}}}`)
	if err == nil || !strings.Contains(err.Error(), "inbox.curator") || !strings.Contains(err.Error(), "warn") {
		t.Fatalf("err = %v, want a refusal that names inbox.curator and warn", err)
	}
}

func TestInboxTrailerStage_RefusesAnUnknownTrailerKey(t *testing.T) {
	if _, err := loadPolicyText(t, `{"inbox": {"curator": {"trailer": {"stages": "enforce"}}}}`); err == nil {
		t.Fatal("an unknown key under inbox.curator.trailer must be refused")
	}
}

func TestInboxTrailerPolicy_DirectStructConfig(t *testing.T) {
	p := policy.Policy{
		Inbox: &policy.InboxPolicy{
			Curator: &policy.InboxCuratorPolicy{
				Trailer: &policy.InboxTrailerPolicy{Stage: "enforce"},
			},
		},
	}
	if got := p.InboxTrailerStage(); got != "enforce" {
		t.Errorf("InboxTrailerStage() = %q, want enforce", got)
	}
}

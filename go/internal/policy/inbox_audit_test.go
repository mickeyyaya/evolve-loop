package policy_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestInboxAuditConfig_AnAbsentBlockIsTheDefault(t *testing.T) {
	want := policy.InboxAuditConfig{MinAgeDays: 14, TriggerCount: 40, MaxItems: 20}
	for name, body := range map[string]string{"no inbox block": `{}`, "no audit block": `{"inbox": {"curator": {"stage": "shadow"}}}`} {
		p, err := loadPolicyText(t, body)
		if err != nil {
			t.Fatal(err)
		}
		if got := p.InboxAuditConfig(); got != want {
			t.Errorf("%s: InboxAuditConfig() = %+v, want %+v", name, got, want)
		}
	}
}

func TestInboxAuditConfig_EveryNamedValueOverridesTheDefault(t *testing.T) {
	p, err := loadPolicyText(t, `{"inbox": {"curator": {"audit": {"min_age_days": 30, "trigger_count": 5, "max_items": 3}}}}`)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := p.InboxAuditConfig(), (policy.InboxAuditConfig{MinAgeDays: 30, TriggerCount: 5, MaxItems: 3}); got != want {
		t.Fatalf("InboxAuditConfig() = %+v, want %+v", got, want)
	}
}

func TestInboxAuditPolicy_RefusesAValueThatIsNotPositive(t *testing.T) {
	for name, tc := range map[string]struct{ body, why string }{
		"a zero age":         {`{"min_age_days": 0}`, "min_age_days"},
		"a negative trigger": {`{"trigger_count": -1}`, "trigger_count"},
		"a zero item cap":    {`{"max_items": 0}`, "max_items"},
		"a count as text":    {`{"max_items": "20"}`, "max_items"},
	} {
		_, err := loadPolicyText(t, `{"inbox": {"curator": {"audit": `+tc.body+`}}}`)
		if err == nil || !strings.Contains(err.Error(), "inbox.curator: audit: ") || !strings.Contains(err.Error(), tc.why) {
			t.Errorf("%s: err = %v, want an error that names inbox.curator: audit: and %s", name, err, tc.why)
		}
	}
}

func TestInboxAuditPolicy_RefusesAnUnknownKey(t *testing.T) {
	_, err := loadPolicyText(t, `{"inbox": {"curator": {"audit": {"apply": true}}}}`)
	if err == nil || !strings.Contains(err.Error(), "inbox.curator.audit.apply") {
		t.Errorf("err = %v, want an error that names inbox.curator.audit.apply", err)
	}
}

package policy_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestInboxCuratorConfig_AnAbsentBlockIsTheShadowDefault(t *testing.T) {
	got := policy.Policy{}.InboxCuratorConfig()

	want := policy.InboxCuratorConfig{
		Stage: "shadow", OverlapRefuse: 0.5, OverlapWarn: 0.3, ConsumeMinShare: 0.5,
		Kinds: []string{"bug", "docs", "feature", "fix", "hardening", "maintenance", "pipeline-integrity", "pipeline-repair", "refactor", "research", "security", "sweep", "task", "techdebt", "test"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("InboxCuratorConfig() = %+v, want %+v", got, want)
	}
	if got.Enforced() || !(policy.InboxCuratorConfig{Stage: "enforce"}).Enforced() {
		t.Errorf("Enforced() must be true only for the enforce stage")
	}
}

func TestInboxCuratorConfig_EveryNamedValueOverridesTheDefault(t *testing.T) {
	p, err := loadPolicyText(t, `{"inbox": {"curator": {
		"stage": "enforce", "overlap_refuse": 0.7, "overlap_warn": 0.2, "consume_min_share": 1, "kinds": ["bug", "feature"]
	}}}`)
	if err != nil {
		t.Fatal(err)
	}

	got := p.InboxCuratorConfig()

	want := policy.InboxCuratorConfig{Stage: "enforce", OverlapRefuse: 0.7, OverlapWarn: 0.2, ConsumeMinShare: 1, Kinds: []string{"bug", "feature"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("InboxCuratorConfig() = %+v, want %+v", got, want)
	}
}

func TestInboxCuratorConfig_AnEmptyInboxBlockKeepsTheDefault(t *testing.T) {
	p, err := loadPolicyText(t, `{"inbox": {}}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.InboxCuratorConfig(); got.Stage != "shadow" || got.OverlapRefuse != 0.5 {
		t.Fatalf("InboxCuratorConfig() = %+v, want the default", got)
	}
}

func TestInboxCuratorPolicy_RefusesAnInvalidValueNamingTheBlock(t *testing.T) {
	for name, tc := range map[string]struct{ body, why string }{
		"an unknown stage":          {`{"stage": "off"}`, `stage "off"`},
		"a refuse threshold over 1": {`{"overlap_refuse": 1.5}`, "overlap_refuse"},
		"a zero refuse threshold":   {`{"overlap_refuse": 0}`, "overlap_refuse"},
		"a negative warn threshold": {`{"overlap_warn": -0.1}`, "overlap_warn"},
		"warn above refuse":         {`{"overlap_refuse": 0.4, "overlap_warn": 0.6}`, "overlap_warn"},
		"warn above default refuse": {`{"overlap_warn": 0.6}`, "overlap_warn"},
		"a zero consume share":      {`{"consume_min_share": 0}`, "consume_min_share"},
		"an empty kind list":        {`{"kinds": []}`, "kinds"},
		"a kind listed twice":       {`{"kinds": ["bug", "bug"]}`, "kinds"},
		"an unknown key":            {`{"stagee": "shadow"}`, "stagee"},
		"a stage of the wrong type": {`{"stage": 3}`, "stage"},
	} {
		_, err := loadPolicyText(t, `{"inbox": {"curator": `+tc.body+`}}`)
		if err == nil || !strings.Contains(err.Error(), "inbox.curator") || !strings.Contains(err.Error(), tc.why) {
			t.Errorf("%s: err = %v, want a refusal naming inbox.curator and %q", name, err, tc.why)
		}
	}
}

func TestInboxCuratorConfig_TheCheckedInPolicyNamesTheShadowStage(t *testing.T) {
	p, err := policy.Load(filepath.Join("..", "..", "..", ".evolve", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.InboxCuratorConfig(); p.Inbox == nil || p.Inbox.Curator == nil || got.Stage != "shadow" {
		t.Fatalf("the checked-in policy must name inbox.curator with stage shadow, got %+v", got)
	}
}

func TestInboxCuratorConfig_ABlockBuiltInGoResolvesAsTheDecodedOne(t *testing.T) {
	refuse := 0.8
	p := policy.Policy{Inbox: &policy.InboxPolicy{Curator: &policy.InboxCuratorPolicy{Stage: "enforce", OverlapRefuse: &refuse}}}

	got := p.InboxCuratorConfig()

	if got.Stage != "enforce" || got.OverlapRefuse != 0.8 || got.OverlapWarn != 0.3 || len(got.Kinds) == 0 {
		t.Fatalf("InboxCuratorConfig() = %+v, want enforce, 0.8 and the default warn and kinds", got)
	}
}

func TestInboxCuratorConfig_ALowRefuseWithNoWarnPullsTheWarnDownToIt(t *testing.T) {
	p, err := loadPolicyText(t, `{"inbox": {"curator": {"overlap_refuse": 0.2}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.InboxCuratorConfig(); got.OverlapWarn != 0.2 {
		t.Fatalf("OverlapWarn = %v, want 0.2 (never above the refuse threshold)", got.OverlapWarn)
	}
}

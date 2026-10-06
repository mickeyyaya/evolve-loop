package policy_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

var operatorClassOrder = []string{"correctness", "stability", "performance", "debuggability", "feature", "maintainability", "hygiene", "security"}

func TestInboxPriorityConfig_AnAbsentBlockIsTheCompiledDefault(t *testing.T) {
	got := policy.Policy{}.InboxPriorityConfig()

	want := policy.InboxPriorityConfig{
		ClassOrder:      operatorClassOrder,
		Factors:         policy.InboxPriorityFactors{Base: 0.45, Class: 0.20, Unblocks: 0.15, Recurrence: 0.10, Age: 0.05, Goal: 0.05},
		AgeHalflifeDays: 30,
		UnblocksCap:     3,
		RecurrenceCap:   5,
		PreemptMargin:   0.05,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("InboxPriorityConfig() = %+v, want %+v", got, want)
	}
}

func TestInboxPriorityConfig_TheDefaultFactorsSumToOne(t *testing.T) {
	f := policy.Policy{}.InboxPriorityConfig().Factors

	if sum := f.Base + f.Class + f.Unblocks + f.Recurrence + f.Age + f.Goal; sum < 0.999999 || sum > 1.000001 {
		t.Fatalf("default factors sum to %v; a score must stay in [0, 1]", sum)
	}
}

func TestInboxPriorityConfig_EveryNamedValueOverridesTheDefault(t *testing.T) {
	p, err := loadPolicyText(t, `{"inbox_priority": {
		"class_order": ["hygiene", "security"],
		"factors": {"base": 1, "class": 0, "unblocks": 0, "recurrence": 0, "age": 0, "goal": 0.5},
		"age_halflife_days": 7,
		"unblocks_cap": 2,
		"recurrence_cap": 9,
		"active_campaigns": ["inbox-priority"],
		"preempt_margin": 0
	}}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := p.InboxPriorityConfig()

	want := policy.InboxPriorityConfig{
		ClassOrder:      []string{"hygiene", "security"},
		Factors:         policy.InboxPriorityFactors{Base: 1, Goal: 0.5},
		AgeHalflifeDays: 7,
		UnblocksCap:     2,
		RecurrenceCap:   9,
		ActiveCampaigns: []string{"inbox-priority"},
		PreemptMargin:   0,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("InboxPriorityConfig() = %+v, want %+v", got, want)
	}
}

func TestInboxPriorityConfig_ReturnsACopyTheCallerCannotCorrupt(t *testing.T) {
	p, err := loadPolicyText(t, `{"inbox_priority": {"class_order": ["security", "hygiene"], "active_campaigns": ["x"]}}`)
	if err != nil {
		t.Fatal(err)
	}
	first := p.InboxPriorityConfig()
	first.ClassOrder[0], first.ActiveCampaigns[0] = "mutated", "mutated"
	def := policy.Policy{}.InboxPriorityConfig()
	def.ClassOrder[0] = "mutated"

	if again := p.InboxPriorityConfig(); again.ClassOrder[0] != "security" || again.ActiveCampaigns[0] != "x" {
		t.Errorf("a caller's edit leaked into the policy: %+v", again)
	}
	if again := (policy.Policy{}).InboxPriorityConfig(); again.ClassOrder[0] != "correctness" {
		t.Errorf("a caller's edit leaked into the compiled default: %v", again.ClassOrder)
	}
}

func TestLoad_InboxPriorityRefusesAnUnknownKey(t *testing.T) {
	for name, text := range map[string]string{
		"in the block":   `{"inbox_priority": {"class_ordr": ["security"]}}`,
		"in the factors": `{"inbox_priority": {"factors": {"base": 1, "class": 0, "unblocks": 0, "recurrence": 0, "age": 0, "goal": 0, "urgency": 1}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := loadPolicyText(t, text)

			if err == nil || !strings.Contains(err.Error(), "inbox_priority") {
				t.Fatalf("a typo inside the block must fail the load naming the block, got %v", err)
			}
		})
	}
}

func TestLoad_InboxPriorityRefusesAFactorsBlockThatOmitsAFactor(t *testing.T) {
	all := map[string]string{"base": "0.4", "class": "0.2", "unblocks": "0.2", "recurrence": "0.1", "age": "0.05", "goal": "0.05"}
	for omitted := range all {
		t.Run(omitted, func(t *testing.T) {
			var pairs []string
			for name, value := range all {
				if name != omitted {
					pairs = append(pairs, `"`+name+`": `+value)
				}
			}

			_, err := loadPolicyText(t, `{"inbox_priority": {"factors": {`+strings.Join(pairs, ", ")+`}}}`)

			if err == nil || !strings.Contains(err.Error(), `"`+omitted+`"`) {
				t.Fatalf("an omitted factor must not silently weigh 0, got %v", err)
			}
		})
	}
}

func TestLoad_InboxPriorityRefusesAnInvalidValue(t *testing.T) {
	for name, tc := range map[string]struct{ block, why string }{
		"an empty class order":       {`{"class_order": []}`, "class_order"},
		"a blank class":              {`{"class_order": ["security", " "]}`, "class_order"},
		"a padded class":             {`{"class_order": [" security"]}`, "class_order"},
		"a class listed twice":       {`{"class_order": ["security", "hygiene", "security"]}`, "twice"},
		"a blank campaign":           {`{"active_campaigns": [""]}`, "active_campaigns"},
		"a negative factor":          {`{"factors": {"base": -0.1, "class": 0, "unblocks": 0, "recurrence": 0, "age": 0, "goal": 1}}`, "base"},
		"every factor zero":          {`{"factors": {"base": 0, "class": 0, "unblocks": 0, "recurrence": 0, "age": 0, "goal": 0}}`, "factors"},
		"a zero half-life":           {`{"age_halflife_days": 0}`, "age_halflife_days"},
		"a zero unblocks cap":        {`{"unblocks_cap": 0}`, "unblocks_cap"},
		"a negative recurrence cap":  {`{"recurrence_cap": -1}`, "recurrence_cap"},
		"a negative preempt margin":  {`{"preempt_margin": -0.01}`, "preempt_margin"},
		"a mistyped class order":     {`{"class_order": "security"}`, "class_order"},
		"a block that is not object": {`["security"]`, "inbox_priority"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := loadPolicyText(t, `{"inbox_priority": `+tc.block+`}`)

			if err == nil || !strings.Contains(err.Error(), "inbox_priority") || !strings.Contains(err.Error(), tc.why) {
				t.Fatalf("err = %v, want a load failure naming inbox_priority and %q", err, tc.why)
			}
		})
	}
}

func TestInboxPriorityPolicy_UnmarshalJSONIsStrict(t *testing.T) {
	var b policy.InboxPriorityPolicy
	if err := b.UnmarshalJSON([]byte(`{"class_order": ["security"]}`)); err != nil || !reflect.DeepEqual(b.ClassOrder, []string{"security"}) {
		t.Fatalf("UnmarshalJSON = %v, decoded %+v", err, b)
	}
	if err := b.UnmarshalJSON([]byte(`{"weights": {}}`)); err == nil {
		t.Fatal("an unknown key must be refused")
	}
	var f policy.InboxPriorityFactors
	if err := f.UnmarshalJSON([]byte(`{"base": 1}`)); err == nil {
		t.Fatal("a factors block naming one factor must be refused")
	}
	if err := f.UnmarshalJSON([]byte(`"base"`)); err == nil {
		t.Fatal("a factors block that is not an object must be refused")
	}
}

func TestLoad_TheCheckedInPolicyCarriesTheOperatorsClassOrder(t *testing.T) {
	p, err := policy.Load(filepath.Join("..", "..", "..", ".evolve", "policy.json"))
	if err != nil {
		t.Fatalf("the checked-in policy must load: %v", err)
	}

	got := p.InboxPriorityConfig()

	if p.InboxPriority == nil || !reflect.DeepEqual(got.ClassOrder, operatorClassOrder) {
		t.Fatalf("checked-in class_order = %v, want the operator's corrected order, security last, %v", got.ClassOrder, operatorClassOrder)
	}
	if p.InboxPriority.Factors == nil {
		t.Error("the checked-in block must name the factor weights, so they live in config rather than in the compiled default")
	}
}

func TestLoad_ANullInboxPriorityBlockIsTheCompiledDefault(t *testing.T) {
	p, err := loadPolicyText(t, `{"inbox_priority": null}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := p.InboxPriorityConfig(); p.InboxPriority != nil || !reflect.DeepEqual(got, policy.Policy{}.InboxPriorityConfig()) {
		t.Fatalf("a null block = %+v resolving to %+v, want the compiled default an absent block gives", p.InboxPriority, got)
	}
}

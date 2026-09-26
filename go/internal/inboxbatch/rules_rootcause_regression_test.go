package inboxbatch

import (
	"fmt"
	"strings"
	"testing"
)

// wantDefaultRuleTypes pins order, which is presentation-only, so a substitution fails as loudly as an addition.
var wantDefaultRuleTypes = []string{
	"inboxbatch.campaignRule",
	"inboxbatch.fileAreaRule",
	"inboxbatch.depRule",
}

func TestDefaultRules_DoesNotBindOnRootCauseProse(t *testing.T) {
	rules := DefaultRules()

	if got := len(rules); got != len(wantDefaultRuleTypes) {
		t.Fatalf("DefaultRules() returned %d rules, want %d %v — cycle-1204 audit D1/D2 rejected adding a "+
			"free-form-prose rule here. A 4th default-on rule needs a discriminative bound "+
			"(a hubAreaMaxItems-style ceiling or a minAreaDepth-style floor) AND a non-tautological "+
			"eval against real .evolve/inbox data before it may default on.",
			got, len(wantDefaultRuleTypes), wantDefaultRuleTypes)
	}
	for i, r := range rules {
		if got := ruleTypeName(r); got != wantDefaultRuleTypes[i] {
			t.Errorf("DefaultRules()[%d] is %s, want %s — a structural rule was replaced; "+
				"see the D1/D2 bar above before changing the default set", i, got, wantDefaultRuleTypes[i])
		}
	}

	// Item has no root_cause field, so Title stands in as the free-form prose; no case shares anything else.
	const prose = "verdict incoherence under contention: the tier reported RED because SubstantiveError was never populated"

	for _, tc := range []struct {
		name  string
		items []Item
		why   string
	}{
		{
			name: "identical-prose",
			items: []Item{
				{ID: "a-item", Title: prose},
				{ID: "b-item", Title: prose},
				{ID: "c-item", Title: prose},
			},
			why: "exact-match prose binding is the rejected design itself (D1)",
		},
		{
			name: "case-and-whitespace-variants",
			items: []Item{
				{ID: "a-item", Title: "Quota Regex Drift"},
				{ID: "b-item", Title: "quota regex drift"},
				{ID: "c-item", Title: "  QUOTA   REGEX drift  "},
				{ID: "d-item", Title: "\tquota\tregex\tdrift\n"},
			},
			why: "no default rule may derive grouping from prose, normalised or not (D2)",
		},
		{
			name: "empty-and-whitespace-only-prose",
			items: []Item{
				{ID: "a-item", Title: ""},
				{ID: "b-item", Title: ""},
				{ID: "c-item", Title: "   "},
				{ID: "d-item", Title: "\n\t"},
			},
			why: "an empty key must never become a grouping bucket",
		},
		{
			name: "shared-prose-plus-one-outlier",
			items: []Item{
				{ID: "a-item", Title: prose},
				{ID: "b-item", Title: prose},
				{ID: "c-item", Title: "unrelated: fleet lane width is a hard commitment"},
			},
			why: "partial prose overlap must bind nothing at all",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, r := range rules {
				if edges := r.Edges(tc.items); len(edges) != 0 {
					t.Errorf("%s bound %d edge(s) on items sharing only a free-form prose field: %+v — %s",
						ruleTypeName(r), len(edges), edges, tc.why)
				}
			}
		})
	}
}

// ruleTypeName strips a leading "*" so a pointer-receiver rule still matches the composition list.
func ruleTypeName(r Rule) string {
	return strings.TrimPrefix(fmt.Sprintf("%T", r), "*")
}

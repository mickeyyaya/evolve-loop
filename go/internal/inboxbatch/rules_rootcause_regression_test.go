package inboxbatch

// rules_rootcause_regression_test.go — the guard-rail against reintroducing the
// cycle-1204 audit-REJECTED root-cause binding design.
//
// Cycle-1204 proposed a `rootCauseRule` that bound inbox items by exact string
// equality on a free-form prose field (`root_cause`) and placed it in
// DefaultRules(), i.e. default-on for every backlog. The audit rejected it on
// two grounds, and the production code never landed:
//
//	D1 (no-op on real data): measured against the 67 live .evolve/inbox items,
//	all 20 non-empty root_cause values were UNIQUE prose (median 317 bytes).
//	Exact-match grouping over unique strings binds nothing — the rule paid
//	rule-set complexity for zero edges.
//
//	D2 (unbounded fusion): it carried neither discriminative guard its siblings
//	have — no hubAreaMaxItems-style CEILING, no minAreaDepth-style FLOOR. The
//	moment a normalising producer landed upstream (lowercase, collapse
//	whitespace, truncate), the campaign-less backlog would collapse into one
//	over-fused cluster, the exact mega-cluster pathology hubAreaMaxItems and
//	minAreaDepth were each introduced to kill (see rules_area_floor_test.go).
//
// So there is no feature to regression-test; the regression worth pinning is
// DEFENSIVE. DefaultRules() must stay the three bounded structural signals
// (campaign = explicit operator declaration, file-area = ceiling AND floor,
// deps = hard structural references), and none of them may derive grouping from
// a shared free-form prose field.
//
// `Item` has no `root_cause` field (it never landed), so the prose analogue
// asserted here is `Item.Title`: the live field that is genuinely
// unstructured, author-written and not a vocabulary (Class/Priority/Kind are
// enums). It is the closest faithful stand-in for the rejected field, and it
// keeps the guard meaningful without adding the field the audit rejected.
//
// This test is intentionally hostile to a future 4th default rule. That is not
// a ban on ever adding one — it is a demand that adding one come with a
// discriminative bound and a non-tautological eval against real backlog data,
// which is precisely what D1/D2 found missing.

import (
	"fmt"
	"strings"
	"testing"
)

// wantDefaultRuleTypes is the audited DefaultRules() composition, in order.
// Order is presentation-only (Classify unions edges), but pinning it makes a
// substitution — swapping a structural rule for a prose rule while keeping the
// count at three — as loud as an addition.
var wantDefaultRuleTypes = []string{
	"inboxbatch.campaignRule",
	"inboxbatch.fileAreaRule",
	"inboxbatch.depRule",
}

// TestDefaultRules_DoesNotBindOnRootCauseProse pins both halves of the
// contract: the rule set IS exactly the three bounded structural rules
// (composition), and no rule in it binds items whose only commonality is a
// free-form prose field (behaviour). Composition alone would miss a prose rule
// that replaced a structural one; behaviour alone would miss a prose rule that
// happens to be a no-op on this test's inputs. Both, together, are what make
// the guard load-bearing.
func TestDefaultRules_DoesNotBindOnRootCauseProse(t *testing.T) {
	rules := DefaultRules()

	// --- composition ------------------------------------------------------
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

	// --- behaviour --------------------------------------------------------
	// Every case below shares ONLY prose: no Campaign, no Files, no Deps, so
	// the three structural rules have nothing legitimate to bind on and the
	// correct answer is always zero edges.
	const prose = "verdict incoherence under contention: the tier reported RED because SubstantiveError was never populated"

	for _, tc := range []struct {
		name  string
		items []Item
		why   string
	}{
		{
			// The rejected design's happy path: byProse[exact] buckets these
			// three together and emits a spanning chain.
			name: "identical-prose",
			items: []Item{
				{ID: "a-item", Title: prose},
				{ID: "b-item", Title: prose},
				{ID: "c-item", Title: prose},
			},
			why: "exact-match prose binding is the rejected design itself (D1)",
		},
		{
			// D2's failure mode: the shape a normalising producer would emit.
			// A rule that normalises before bucketing fuses all four.
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
			// The degenerate case: an empty key must never become a bucket.
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
			// Mixed: one distinct item must not be dragged in either.
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

// ruleTypeName renders a Rule's concrete type as package.Type, tolerating a
// pointer receiver so a future *fooRule reads as inboxbatch.fooRule rather
// than failing the composition check for an unrelated reason.
func ruleTypeName(r Rule) string {
	return strings.TrimPrefix(fmt.Sprintf("%T", r), "*")
}

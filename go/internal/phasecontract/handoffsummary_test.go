package phasecontract

import "testing"

// handoffsummary_test.go — RED contract for cycle-565 Slice S1 of
// report-size-contracts-jit-artifacts (this fleet lane's sole triage-committed
// top_n task; see triage-report.md). The contract-gate gains a canonical,
// never-evict "Handoff Summary" section (decisions, acceptance criteria, open
// questions, verdicts) for the report families the triage decision names —
// build, scout, audit — so its size can be separately budgeted
// (see reportsize_test.go in go/internal/deliverable) without disturbing the
// existing per-phase Sections the classifiers already require.
//
// RED today: phasecontract.HandoffSummary does not exist (compile failure).

func TestHandoffSummarySection_Canonical(t *testing.T) {
	if HandoffSummary.Canonical != "## Handoff Summary" {
		t.Fatalf("HandoffSummary.Canonical = %q, want %q", HandoffSummary.Canonical, "## Handoff Summary")
	}
	if !HandoffSummary.Present("intro\n## Handoff Summary\ndecisions\n") {
		t.Error("HandoffSummary must be Present() when the canonical heading occurs")
	}
	if HandoffSummary.Present("## Something Else\n") {
		t.Error("HandoffSummary must not be Present() when the heading is absent")
	}
}

func sectionsHave(sections []Section, canonical string) bool {
	for _, s := range sections {
		if s.Canonical == canonical {
			return true
		}
	}
	return false
}

// TestBuildScoutAudit_RequireHandoffSummary pins the triage-committed scope:
// build/scout/audit gain the requirement this cycle (S1). Once this passes,
// the EXISTING TestProducersDeclareCanonical (contract_test.go) will itself
// start requiring the producer templates (evolve-builder-reference.md,
// evolve-scout-reference.md, evolve-auditor-reference.md) to declare the new
// heading too — no duplicate template-drift test needed here.
func TestBuildScoutAudit_RequireHandoffSummary(t *testing.T) {
	for _, tc := range []struct {
		name string
		r    Report
	}{
		{"build", Build},
		{"scout", Scout},
		{"audit", Audit},
	} {
		if !sectionsHave(tc.r.Sections, HandoffSummary.Canonical) {
			t.Errorf("%s.Sections must include HandoffSummary (%q); got %+v", tc.name, HandoffSummary.Canonical, tc.r.Sections)
		}
	}
}

// TestTDDIntentTriage_NotExpanded is the scope-boundary negative test: tdd/
// intent/triage are explicitly OUT of Slice S1 (the triage decision names only
// build/scout/audit). This is the strongest anti-scope-creep signal — a naive
// "add HandoffSummary everywhere" implementation fails it.
func TestTDDIntentTriage_NotExpanded(t *testing.T) {
	for _, tc := range []struct {
		name string
		r    Report
	}{
		{"tdd", TDD},
		{"intent", Intent},
		{"triage", Triage},
	} {
		if sectionsHave(tc.r.Sections, HandoffSummary.Canonical) {
			t.Errorf("%s.Sections must NOT include HandoffSummary this slice (S1 scopes build/scout/audit only); got %+v", tc.name, tc.r.Sections)
		}
	}
}

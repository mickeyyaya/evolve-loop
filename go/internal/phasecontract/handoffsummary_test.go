package phasecontract

import "testing"

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

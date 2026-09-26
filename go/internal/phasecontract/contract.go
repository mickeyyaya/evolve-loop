// Package phasecontract is the single source of truth for each phase's deliverable
// contract: report headings, artifact location and shape, and the verdict sentinel.
// See docs/architecture/packages/internal-phasecontract.md.
package phasecontract

import "strings"

// Section is one required report region; Accepted lists Canonical first, then tolerated legacy variants.
type Section struct {
	Canonical string
	Accepted  []string
}

// Title is the heading text without its `## ` marker, the form reportdoc.HasSection matches.
func (s Section) Title() string { return strings.TrimPrefix(s.Canonical, "## ") }

// Present reports whether any Accepted variant occurs in content.
func (s Section) Present(content string) bool {
	for _, v := range s.Accepted {
		if strings.Contains(content, v) {
			return true
		}
	}
	return false
}

// Report is a phase's heading contract; Producers are the agents/*.md basenames that must declare each Canonical.
type Report struct {
	Phase     string
	Sections  []Section
	Producers []string
}

// Complete reports whether every always-on section is present; staged sections are skipped.
func (r Report) Complete(content string) bool {
	for _, s := range r.Sections {
		if stagedSections[s.Canonical] {
			continue
		}
		if !s.Present(content) {
			return false
		}
	}
	return true
}

// HandoffSummary is the never-evict summary section that the report-size gate budgets separately.
var HandoffSummary = Section{Canonical: "## Handoff Summary", Accepted: []string{"## Handoff Summary"}}

// Build requires a changed-files section plus the HandoffSummary.
var Build = Report{
	Phase: "build",
	Sections: []Section{
		{Canonical: "## Changes", Accepted: []string{"## Changes", "## Files Changed", "## Files Modified"}},
		HandoffSummary,
	},
	Producers: []string{"evolve-builder-reference"},
}

// Scout requires the tasks heading plus the HandoffSummary; the "at least one task" check stays in scout.go.
var Scout = Report{
	Phase: "scout",
	Sections: []Section{
		{Canonical: "## Selected Tasks", Accepted: []string{"## Selected Tasks", "## Proposed Tasks"}},
		HandoffSummary,
	},
	Producers: []string{"evolve-scout", "evolve-scout-reference"},
}

// TDD requires an acceptance section and a RED-run section.
var TDD = Report{
	Phase: "tdd",
	Sections: []Section{
		{Canonical: "## AC-Materialization", Accepted: []string{"## AC-Materialization", "## Acceptance", "## Coverage Map"}},
		{Canonical: "## RED Run Output", Accepted: []string{"## RED Run Output", "## RED Tests", "## Test Files Written"}},
	},
	Producers: []string{"evolve-tdd-engineer"},
}

// ExplanationDocumentation is the audit section owed only while the explanation-documentation contract is active.
var ExplanationDocumentation = Section{Canonical: "## Explanation Documentation", Accepted: []string{"## Explanation Documentation"}}

// Audit requires a Verdict heading plus the HandoffSummary; the classifier extracts the verdict token.
var Audit = Report{
	Phase: "audit",
	Sections: []Section{
		{Canonical: "## Verdict", Accepted: []string{"## Verdict", "Verdict:"}},
		HandoffSummary,
	},
	Producers: []string{"evolve-auditor-reference"},
}

// Intent requires the goal and acceptance_checks line tokens, which are not markdown headings.
var Intent = Report{
	Phase: "intent",
	Sections: []Section{
		{Canonical: "goal:", Accepted: []string{"goal:"}},
		{Canonical: "acceptance_checks:", Accepted: []string{"acceptance_checks:"}},
	},
	Producers: []string{"evolve-intent"},
}

// Triage requires the top_n heading; the "at least one item" check stays in triage.go.
var Triage = Report{
	Phase:     "triage",
	Sections:  []Section{{Canonical: "## top_n", Accepted: []string{"## top_n"}}},
	Producers: []string{"evolve-triage"},
}

// All is every built-in phase Report.
var All = []Report{Build, Scout, TDD, Audit, Intent, Triage}

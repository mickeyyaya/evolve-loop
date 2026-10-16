// Package phasecontract is the single source of truth for each phase's deliverable
// contract: report headings, artifact location and shape, and the verdict sentinel.
// See docs/architecture/packages/internal-phasecontract.md.
package phasecontract

import "strings"

type Section struct {
	Canonical string
	Accepted  []string
}

func (s Section) Title() string { return strings.TrimPrefix(s.Canonical, "## ") }

func (s Section) Present(content string) bool {
	for _, v := range s.Accepted {
		if strings.Contains(content, v) {
			return true
		}
	}
	return false
}

type Report struct {
	Phase     string
	Sections  []Section
	Producers []string
}

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

var HandoffSummary = Section{Canonical: "## Handoff Summary", Accepted: []string{"## Handoff Summary"}}

var Build = Report{
	Phase: "build",
	Sections: []Section{
		{Canonical: "## Changes", Accepted: []string{"## Changes", "## Files Changed", "## Files Modified"}},
		HandoffSummary,
	},
	Producers: []string{"evolve-builder-reference"},
}

var SelectedTasks = Section{Canonical: "## Selected Tasks", Accepted: []string{"## Selected Tasks", "## Proposed Tasks"}}

var Scout = Report{
	Phase: "scout",
	Sections: []Section{
		SelectedTasks,
		HandoffSummary,
	},
	Producers: []string{"evolve-scout", "evolve-scout-reference"},
}

var TDD = Report{
	Phase: "tdd",
	Sections: []Section{
		{Canonical: "## AC-Materialization", Accepted: []string{"## AC-Materialization", "## Acceptance", "## Coverage Map"}},
		{Canonical: "## RED Run Output", Accepted: []string{"## RED Run Output", "## RED Tests", "## Test Files Written"}},
	},
	Producers: []string{"evolve-tdd-engineer"},
}

var ExplanationDocumentation = Section{Canonical: "## Explanation Documentation", Accepted: []string{"## Explanation Documentation"}}

var Audit = Report{
	Phase: "audit",
	Sections: []Section{
		{Canonical: "## Verdict", Accepted: []string{"## Verdict", "Verdict:"}},
		HandoffSummary,
	},
	Producers: []string{"evolve-auditor-reference"},
}

var Intent = Report{
	Phase: "intent",
	Sections: []Section{
		{Canonical: "goal:", Accepted: []string{"goal:"}},
		{Canonical: "acceptance_checks:", Accepted: []string{"acceptance_checks:"}},
	},
	Producers: []string{"evolve-intent"},
}

var Triage = Report{
	Phase:     "triage",
	Sections:  []Section{{Canonical: "## top_n", Accepted: []string{"## top_n"}}},
	Producers: []string{"evolve-triage"},
}

var All = []Report{Build, Scout, TDD, Audit, Intent, Triage}

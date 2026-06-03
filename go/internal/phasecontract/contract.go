// Package phasecontract is the SINGLE SOURCE OF TRUTH for the report section
// headings each phase's verdict classifier requires.
//
// Both sides of the producer→consumer contract read from here:
//
//   - the CONSUMER: each phase's Go classifier (build/scout/tdd/audit/intent/
//     triage) tests an agent report against these headings to derive a verdict;
//   - the PRODUCER-SIDE ALARM: the contract test (contract_test.go) asserts the
//     agent .md template/reference still DECLARES each canonical heading, so a
//     template edit that renames a section fails CI instead of silently
//     false-FAILing a valid report at cycle time.
//
// This closes the failure class behind cycle-192: the classifiers grepped for
// headings the templates no longer emitted, valid reports were classified FAIL,
// and a build false-FAIL tripped the auditor's report-vs-telemetry cross-check
// → no ship. The 64b2d95 fix widened the per-phase regexes into tolerant
// allow-lists but left the heading strings duplicated in 6 Go files with no
// alarm; this package centralizes them and adds the alarm.
//
// Match semantics that the *declarative* phasespec.ClassifyRules cannot express
// (build's OR-of-headings, scout/triage's heading-plus-≥1-item, tdd's
// OR-within-AND, audit's verdict-token extraction) stay in each phase's
// classifier — they are STABLE logic that does not drift. Only the heading
// STRINGS, which DO drift against the templates, live here.
package phasecontract

import "strings"

// Section is one required region of a phase report. Canonical is the exact
// string the producing agent template MUST declare (the contract test asserts
// it). Accepted lists every string the classifier treats as satisfying the
// section — Canonical first, then tolerated legacy variants kept so an
// in-flight report written against an older template still classifies PASS. A
// section is satisfied when ANY Accepted string is present.
type Section struct {
	Canonical string
	Accepted  []string
}

// Present reports whether any Accepted variant occurs in content.
func (s Section) Present(content string) bool {
	for _, v := range s.Accepted {
		if strings.Contains(content, v) {
			return true
		}
	}
	return false
}

// Report is a phase's report-completeness contract: every Section must be
// Present (AND across sections; OR within a section's Accepted set). Producers
// are the agent .md basenames (under agents/, without extension) whose union
// must declare each Section.Canonical.
type Report struct {
	Phase     string
	Sections  []Section
	Producers []string
}

// Complete reports whether every section is present in content. A Report with
// no sections is trivially complete (no structural requirement). Callers handle
// the empty-artifact case separately (it is a distinct FAIL reason).
func (r Report) Complete(content string) bool {
	for _, s := range r.Sections {
		if !s.Present(content) {
			return false
		}
	}
	return true
}

// The six built-in phase report contracts. Heading strings and producer files
// were verified against agents/*.md at v16.2.0 (see contract_test.go, which
// fails if a producer stops declaring a canonical heading).

// Build — a complete build-report declares a changed-files section. The heading
// drifted "## Files Modified" → "## Files Changed" → the current "## Changes"
// (declared in evolve-builder-reference.md). Classifier: any-accepted (OR).
var Build = Report{
	Phase:     "build",
	Sections:  []Section{{Canonical: "## Changes", Accepted: []string{"## Changes", "## Files Changed", "## Files Modified"}}},
	Producers: []string{"evolve-builder-reference"},
}

// Scout — a non-empty backlog under the tasks heading. Drifted "## Proposed
// Tasks" → "## Selected Tasks". The "≥1 task item" check stays in scout.go.
var Scout = Report{
	Phase:     "scout",
	Sections:  []Section{{Canonical: "## Selected Tasks", Accepted: []string{"## Selected Tasks", "## Proposed Tasks"}}},
	Producers: []string{"evolve-scout", "evolve-scout-reference"},
}

// TDD — an acceptance section AND a RED-run section. Both groups drifted; both
// are declared in evolve-tdd-engineer.md. Classifier: OR within each group,
// AND across groups.
var TDD = Report{
	Phase: "tdd",
	Sections: []Section{
		{Canonical: "## AC-Materialization", Accepted: []string{"## AC-Materialization", "## Acceptance", "## Coverage Map"}},
		{Canonical: "## RED Run Output", Accepted: []string{"## RED Run Output", "## RED Tests", "## Test Files Written"}},
	},
	Producers: []string{"evolve-tdd-engineer"},
}

// Audit — declares a Verdict heading; the classifier extracts the PASS/FAIL/
// WARN/SKIPPED token (verdictCanonicalRE/verdictInlineRE in audit.go). Producer
// declares "## Verdict:" in evolve-auditor-reference.md.
var Audit = Report{
	Phase:     "audit",
	Sections:  []Section{{Canonical: "## Verdict", Accepted: []string{"## Verdict", "Verdict:"}}},
	Producers: []string{"evolve-auditor-reference"},
}

// Intent — declares the goal and acceptance_checks YAML-ish line tokens (not
// markdown ## headings). Both declared in evolve-intent.md.
var Intent = Report{
	Phase: "intent",
	Sections: []Section{
		{Canonical: "goal:", Accepted: []string{"goal:"}},
		{Canonical: "acceptance_checks:", Accepted: []string{"acceptance_checks:"}},
	},
	Producers: []string{"evolve-intent"},
}

// Triage — declares the top_n selection heading; the "≥1 item" check stays in
// triage.go.
var Triage = Report{
	Phase:     "triage",
	Sections:  []Section{{Canonical: "## top_n", Accepted: []string{"## top_n"}}},
	Producers: []string{"evolve-triage"},
}

// All is every built-in phase contract, for the contract test to iterate.
var All = []Report{Build, Scout, TDD, Audit, Intent, Triage}

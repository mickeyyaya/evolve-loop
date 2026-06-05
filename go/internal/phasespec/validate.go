package phasespec

import (
	"fmt"
	"regexp"
)

// nameRE constrains a phase name to lowercase kebab-case, matching the built-in
// phase identifiers (scout, build-planner, …). This keeps a user phase name
// safe to use as a filename, agent suffix, and routing token.
var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// canonicalVerdicts mirrors core's verdict set. Duplicated here (not imported)
// because phasespec must not depend on core — core imports phasespec.
var canonicalVerdicts = map[string]bool{"PASS": true, "FAIL": true, "WARN": true, "SKIPPED": true}

// knownCategories is the closed goal-type vocabulary for PhaseSpec.Categories.
// It mirrors the advisor's goal-type classification (micro-phase catalog) so
// category_index buckets line up with cycle goal types. The check is SOFT: an
// unknown category is a lint warning (UnknownCategories), never a load or
// ValidateUserSpec floor violation — metadata must not block execution.
var knownCategories = map[string]bool{
	"bugfix": true, "feature": true, "refactor": true, "security": true,
	"performance": true, "release": true, "docs": true,
}

// UnknownCategories returns the entries of s.Categories that are not in the
// known goal-type vocabulary, in input order. Empty/nil categories → nil.
func UnknownCategories(s PhaseSpec) []string {
	var unknown []string
	for _, c := range s.Categories {
		if !knownCategories[c] {
			unknown = append(unknown, c)
		}
	}
	return unknown
}

// ValidateUserSpec returns human-readable violations for an operator-authored
// phase spec, or nil when valid. It enforces the safety floor for user phases:
// they MUST be optional (a user phase can never displace or satisfy the
// build→audit→ship spine), and only kind:"llm" is executable today.
func ValidateUserSpec(s PhaseSpec) []string {
	var v []string

	if s.Name == "" {
		v = append(v, "name is required")
	} else if !nameRE.MatchString(s.Name) {
		v = append(v, fmt.Sprintf("name %q must be lowercase kebab-case (^[a-z][a-z0-9-]*$)", s.Name))
	}

	if !s.Optional {
		v = append(v, "user phase must be optional:true — it cannot displace or satisfy the build→audit→ship floor")
	}

	switch s.KindOrDefault() {
	case "llm":
		// supported
	case "native", "command":
		v = append(v, fmt.Sprintf("kind %q is reserved but not yet executable — use \"llm\"", s.Kind))
	default:
		v = append(v, fmt.Sprintf("unknown kind %q (expected llm|native|command)", s.Kind))
	}

	if s.Classify != nil && s.Classify.VerdictOnPass != "" && !canonicalVerdicts[s.Classify.VerdictOnPass] {
		v = append(v, fmt.Sprintf("classify.verdict_on_pass %q must be one of PASS/FAIL/WARN/SKIPPED", s.Classify.VerdictOnPass))
	}

	return v
}

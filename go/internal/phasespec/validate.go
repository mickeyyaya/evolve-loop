package phasespec

import (
	"fmt"
	"regexp"
)

// nameRE constrains a phase name to lowercase kebab-case, matching the built-in
// phase identifiers (scout, build-planner, …). This keeps a user phase name
// safe to use as a filename, agent suffix, and routing token.
var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// twoTierNameRE enforces the two-tier naming rule: user/optional phases must be
// multi-word kebab-case. Single-word names are the reserved built-in vocabulary.
var twoTierNameRE = regexp.MustCompile(`^[a-z]+(-[a-z]+)+$`)

// canonicalVerdicts mirrors core's verdict set. Duplicated here (not imported)
// because phasespec must not depend on core — core imports phasespec.
var canonicalVerdicts = map[string]bool{"PASS": true, "FAIL": true, "WARN": true, "SKIPPED": true}

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
	} else if !twoTierNameRE.MatchString(s.Name) {
		v = append(v, fmt.Sprintf("name %q must be multi-word kebab-case for user/optional phases (e.g. my-check); single-word names are reserved for built-in phases", s.Name))
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

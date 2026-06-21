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

// knownCategories is the closed goal-type vocabulary for PhaseSpec.Categories.
// It mirrors the advisor's goal-type classification (micro-phase catalog) so
// category_index buckets line up with cycle goal types. The check is SOFT: an
// unknown category is a lint warning (UnknownCategories), never a load or
// ValidateUserSpec floor violation — metadata must not block execution.
var knownCategories = map[string]bool{
	"bugfix": true, "feature": true, "refactor": true, "security": true,
	"performance": true, "release": true, "docs": true,
	// Domain goal types (domain-phase-catalog.md §3 recipe table).
	"project-management": true, "business-strategy": true, "accounting-close": true,
	"product-discovery": true, "ops-incident": true,
	// 2026 adversarial-pipeline goal types (skills-derived wave): each names a
	// distinct request class the advisor classifies and routes via the
	// evolve-router.md recipe table.
	"concurrency": true, "api-design": true, "data-migration": true,
	"observability": true, "supply-chain": true, "agent-instruction": true,
	"accessibility": true, "frontend-ui": true, "i18n": true,
	// Wave 5 (skills-derived coverage expansion + plan/evaluate design pairing):
	// data/query, cache, fault-tolerance, delivery-semantics, infra-config and
	// stream/batch request classes the advisor classifies and routes.
	"database": true, "caching": true, "resilience": true,
	"messaging": true, "infrastructure": true, "data-pipeline": true,
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

	// The agent name is used as a filename under agents/ (persona write path in
	// `phases create`), so it gets the same kebab-case floor as the phase name —
	// a crafted "../../x" agent must never escape the agents/ directory.
	if s.Agent != "" && !nameRE.MatchString(s.Agent) {
		v = append(v, fmt.Sprintf("agent %q must be lowercase kebab-case (^[a-z][a-z0-9-]*$)", s.Agent))
	}

	if s.Classify != nil && s.Classify.VerdictOnPass != "" && !canonicalVerdicts[s.Classify.VerdictOnPass] {
		v = append(v, fmt.Sprintf("classify.verdict_on_pass %q must be one of PASS/FAIL/WARN/SKIPPED", s.Classify.VerdictOnPass))
	}

	return v
}

// ValidateActivatingFields returns well-formedness violations for the ADR-0058
// transition-activating fields on a spec, or nil when valid. It is the load-time
// validator (Load calls it): the registry is a contract, so a malformed
// activating field fails loudly rather than silently degrading to the literal
// kernel. It checks SHAPE, not presence — an empty field is valid (the byte-
// identical default); requiring a specific field is the registry-guard's job.
//
//   - branching_strategy must be a known strategy (verdict/history/signal) or empty.
//   - on_pass and on_fail are a verdict-branch PAIR: declare both or neither.
//     Next consults them only when both are set, so a half-set is dead config.
func ValidateActivatingFields(s PhaseSpec) []string {
	var v []string
	switch s.BranchingStrategy {
	case "", BranchingVerdict, BranchingHistory, BranchingSignal:
		// known or unset
	default:
		v = append(v, fmt.Sprintf("branching_strategy %q must be one of %s/%s/%s (or empty)",
			s.BranchingStrategy, BranchingVerdict, BranchingHistory, BranchingSignal))
	}
	if (s.OnPass == "") != (s.OnFail == "") {
		v = append(v, fmt.Sprintf("on_pass/on_fail must be declared together (a verdict branch needs both targets); got on_pass=%q on_fail=%q",
			s.OnPass, s.OnFail))
	}
	return v
}

package phasespec

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
)

// nameRE keeps a phase or agent name safe as a filename, agent suffix and routing token.
var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// twoTierNameRE requires user phase names to be multi-word; single words are reserved for built-ins.
var twoTierNameRE = regexp.MustCompile(`^[a-z]+(-[a-z]+)+$`)

// canonicalVerdicts duplicates core's verdict set because core imports phasespec.
var canonicalVerdicts = map[string]bool{"PASS": true, "FAIL": true, "WARN": true, "SKIPPED": true}

// knownCategories is the advisor's goal-type vocabulary. An unknown category only warns,
// because metadata must not block execution.
var knownCategories = map[string]bool{
	"bugfix": true, "feature": true, "refactor": true, "security": true,
	"performance": true, "release": true, "docs": true,
	"project-management": true, "business-strategy": true, "accounting-close": true,
	"product-discovery": true, "ops-incident": true,
	"concurrency": true, "api-design": true, "data-migration": true,
	"observability": true, "supply-chain": true, "agent-instruction": true,
	"accessibility": true, "frontend-ui": true, "i18n": true,
	"database": true, "caching": true, "resilience": true,
	"messaging": true, "infrastructure": true, "data-pipeline": true,
}

// UnknownCategories returns the entries of s.Categories outside the known vocabulary, in input order.
func UnknownCategories(s PhaseSpec) []string {
	var unknown []string
	for _, c := range s.Categories {
		if !knownCategories[c] {
			unknown = append(unknown, c)
		}
	}
	return unknown
}

// ValidateUserSpec returns a user spec's safety-floor violations, or nil; a user phase must be optional so it never displaces the spine.
func ValidateUserSpec(s PhaseSpec) []string {
	return validateUserSpec(s, false)
}

// ValidateUserSpecWithCatalog is ValidateUserSpec, except an overlay of an optional built-in may keep its single-word name.
func ValidateUserSpecWithCatalog(s PhaseSpec, builtin Catalog) []string {
	return validateUserSpec(s, isOptionalBuiltinName(s.Name, builtin))
}

// isOptionalBuiltinName consults the built-in's Optional flag, never the overlay's, so an
// overlay cannot hijack a mandatory spine phase's name.
func isOptionalBuiltinName(name string, builtin Catalog) bool {
	spec, ok := builtin.Get(name)
	return ok && spec.Optional
}

func validateUserSpec(s PhaseSpec, exemptSingleWordFloor bool) []string {
	var v []string
	v = append(v, ValidateOutputsPartition(s)...)

	if s.Name == "" {
		v = append(v, "name is required")
	} else if !nameRE.MatchString(s.Name) {
		v = append(v, fmt.Sprintf("name %q must be lowercase kebab-case (^[a-z][a-z0-9-]*$)", s.Name))
	} else if !exemptSingleWordFloor && !twoTierNameRE.MatchString(s.Name) {
		v = append(v, fmt.Sprintf("name %q must be multi-word kebab-case for user/optional phases (e.g. my-check); single-word names are reserved for built-in phases", s.Name))
	}

	if !s.Optional {
		v = append(v, "user phase must be optional:true — it cannot displace or satisfy the build→audit→ship floor")
	}

	switch s.KindOrDefault() {
	case "llm", "native":
		// "native" dispatches to an in-process Go phase, not a REPL.
	case "command":
		v = append(v, fmt.Sprintf("kind %q is reserved but not yet executable — use \"llm\"", s.Kind))
	default:
		v = append(v, fmt.Sprintf("unknown kind %q (expected llm|native|command)", s.Kind))
	}

	// The agent name becomes a persona filename under agents/ (phases create), so a
	// crafted "../../x" must never escape that directory.
	if s.Agent != "" && !nameRE.MatchString(s.Agent) {
		v = append(v, fmt.Sprintf("agent %q must be lowercase kebab-case (^[a-z][a-z0-9-]*$)", s.Agent))
	}

	if s.Classify != nil && s.Classify.VerdictOnPass != "" && !canonicalVerdicts[s.Classify.VerdictOnPass] {
		v = append(v, fmt.Sprintf("classify.verdict_on_pass %q must be one of PASS/FAIL/WARN/SKIPPED", s.Classify.VerdictOnPass))
	}

	return v
}

// ValidateActivatingFields checks the shape, not the presence, of a spec's transition-activating fields.
func ValidateActivatingFields(s PhaseSpec) []string {
	var v []string
	switch s.BranchingStrategy {
	case "", BranchingVerdict, BranchingHistory, BranchingSignal:
	default:
		v = append(v, fmt.Sprintf("branching_strategy %q must be one of %s/%s/%s (or empty)",
			s.BranchingStrategy, BranchingVerdict, BranchingHistory, BranchingSignal))
	}
	// Next consults the pair only when both are set, so a half-set pair is dead config.
	if (s.OnPass == "") != (s.OnFail == "") {
		v = append(v, fmt.Sprintf("on_pass/on_fail must be declared together (a verdict branch needs both targets); got on_pass=%q on_fail=%q",
			s.OnPass, s.OnFail))
	}
	return v
}

// ValidateOutputsPartition checks that every secondary output is classified exactly once, agent-owed or harness-produced.
// See ADR-0100.
func ValidateOutputsPartition(s PhaseSpec) []string {
	var v []string
	declared := map[string]bool{}
	for _, f := range s.Outputs.Files[min(1, len(s.Outputs.Files)):] {
		declared[filepath.Base(f)] = true
	}
	seen := map[string]string{}
	for _, class := range []struct {
		name    string
		entries []string
	}{{"agent_owed", s.Outputs.AgentOwed}, {"harness_produced", s.Outputs.HarnessProduced}} {
		for _, e := range class.entries {
			switch {
			case e == "" || e != filepath.Base(e):
				v = append(v, fmt.Sprintf("outputs.%s entry %q must be the basename of a declared secondary output", class.name, e))
			case !declared[e]:
				v = append(v, fmt.Sprintf("outputs.%s names %q, which outputs.files does not declare after the primary", class.name, e))
			case seen[e] != "":
				v = append(v, fmt.Sprintf("%q is classified twice (outputs.%s and outputs.%s)", e, seen[e], class.name))
			default:
				seen[e] = class.name
			}
		}
	}
	for base := range declared {
		if seen[base] == "" {
			v = append(v, fmt.Sprintf("secondary output %q is declared but not classified — add it to outputs.agent_owed (the agent writes it) or outputs.harness_produced (a harness component does)", base))
		}
	}
	primary := ""
	if len(s.Outputs.Files) > 0 {
		primary = filepath.Base(s.Outputs.Files[0])
	}
	for owed, source := range s.Outputs.DerivedFrom {
		switch {
		case seen[owed] != "agent_owed":
			v = append(v, fmt.Sprintf("outputs.derived_from names %q, which outputs.agent_owed does not classify — only an agent-owed secondary is derived", owed))
		case source != primary || primary == "":
			v = append(v, fmt.Sprintf("outputs.derived_from derives %q from %q, but the host derives only from the primary output %q", owed, source, primary))
		}
	}
	slices.Sort(v)
	return v
}

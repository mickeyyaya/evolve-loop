package phasespec

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

var twoTierNameRE = regexp.MustCompile(`^[a-z]+(-[a-z]+)+$`)

var canonicalVerdicts = map[string]bool{"PASS": true, "FAIL": true, "WARN": true, "SKIPPED": true}

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

func UnknownCategories(s PhaseSpec) []string {
	var unknown []string
	for _, c := range s.Categories {
		if !knownCategories[c] {
			unknown = append(unknown, c)
		}
	}
	return unknown
}

func ValidateUserSpec(s PhaseSpec) []string {
	return validateUserSpec(s, false)
}

func ValidateUserSpecWithCatalog(s PhaseSpec, builtin Catalog) []string {
	return validateUserSpec(s, isOptionalBuiltinName(s.Name, builtin))
}

func isOptionalBuiltinName(name string, builtin Catalog) bool {
	spec, ok := builtin.Get(name)
	return ok && spec.Optional
}

func validateUserSpec(s PhaseSpec, exemptSingleWordFloor bool) []string {
	var v []string
	v = append(v, ValidateOutputsPartition(s)...)
	v = append(v, ValidateStage(s)...)

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
	case "command":
		v = append(v, fmt.Sprintf("kind %q is reserved but not yet executable — use \"llm\"", s.Kind))
	default:
		v = append(v, fmt.Sprintf("unknown kind %q (expected llm|native|command)", s.Kind))
	}

	if s.Agent != "" && !nameRE.MatchString(s.Agent) {
		v = append(v, fmt.Sprintf("agent %q must be lowercase kebab-case (^[a-z][a-z0-9-]*$)", s.Agent))
	}

	if s.Classify != nil && s.Classify.VerdictOnPass != "" && !canonicalVerdicts[s.Classify.VerdictOnPass] {
		v = append(v, fmt.Sprintf("classify.verdict_on_pass %q must be one of PASS/FAIL/WARN/SKIPPED", s.Classify.VerdictOnPass))
	}

	return append(v, unregisteredGrammars(s)...)
}

func unregisteredGrammars(s PhaseSpec) []string {
	if s.Classify == nil {
		return nil
	}
	var v []string
	for _, name := range s.Classify.Grammars {
		if !slices.Contains(grammarNames, name) {
			v = append(v, fmt.Sprintf("classify.grammars %q is not a registered report grammar (registered: %s)", name, strings.Join(grammarNames, ", ")))
		}
	}
	return v
}

func ValidateActivatingFields(s PhaseSpec) []string {
	var v []string
	switch s.BranchingStrategy {
	case "", BranchingVerdict, BranchingHistory, BranchingSignal:
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

func ValidateStage(s PhaseSpec) []string {
	switch s.Stage {
	case "", StagePreBuild, StagePostBuild:
		return nil
	}
	return []string{fmt.Sprintf("stage %q must be one of %s/%s (or empty)", s.Stage, StagePreBuild, StagePostBuild)}
}

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

const GrammarCodeReviewReport = "code-review-report"

var grammarNames = []string{GrammarCodeReviewReport}

func Grammars() []string { return slices.Clone(grammarNames) }

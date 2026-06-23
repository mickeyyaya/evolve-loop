package phasecontract

import (
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/phasespec"
)

// FromSpec derives a deliverable Contract from a declarative PhaseSpec. This is
// what makes a phase fully definable in config (a registry entry or an operator
// overlay at .evolve/phases/<name>/phase.json) with NO Go change: the same
// well-formedness contract the built-in map hardcodes for the 6 spine phases is
// instead computed from the spec's own fields for user/minted phases.
//
// Derivation is well-formedness ONLY (artifact location, kind, required
// sections) — never a semantic verdict requirement (anti-Goodhart). Verdicts are
// emitted strictly on opt-in (see verdictsFromSpec); the default nil keeps the
// contract gate additive, exactly as the built-in non-audit phases do.
//
// Built-ins remain authoritative: callers (see Resolver) consult the hardcoded
// map first and only fall back to FromSpec on a miss, so a user phase can never
// weaken a spine phase's contract.
func FromSpec(spec phasespec.PhaseSpec) Contract {
	verdicts := verdictsFromSpec(spec)
	return Contract{
		Phase:        spec.Name,
		AgentName:    spec.AgentName(),
		ArtifactName: artifactNameFromSpec(spec),
		Kind:         kindFromArtifact(artifactNameFromSpec(spec)),
		Sections:     sectionsFromClassify(spec),
		Verdicts:     verdicts,
		RequiredKeys: nil,
		WriteTarget:  TargetWorkspace,
		// Opt-in, and only meaningful for verdict-emitting phases (ADR-0039 §7).
		RequireFailureContext: len(verdicts) > 0 && spec.Classify != nil && spec.Classify.RequireFailureContext,
	}
}

// SynthesizesContract reports whether a spec yields a meaningful derived
// contract: only "llm"-kind phases (an agent actually writes the artifact) or
// specs that explicitly declare outputs.files. Native/command executors with
// no declared outputs get NO synthesized contract — inventing <name>-report.md
// for a deterministic executor produced the unsatisfiable ship contract that
// tripped the enforce gate 3× and forced a breaker demotion every shipping
// cycle (cycle-281). This predicate is the single home of the rule; Resolve
// and any projection (inventory, lint) must consult it before FromSpec.
func SynthesizesContract(spec phasespec.PhaseSpec) bool {
	if spec.KindOrDefault() == "llm" {
		return true
	}
	return len(spec.Outputs.Files) > 0 && spec.Outputs.Files[0] != ""
}

// artifactNameFromSpec returns the bare filename the phase writes. It takes only
// the basename of outputs.files[0] (stripping any cycle-{cycle}/ template path),
// matching the built-in convention of storing bare filenames. Falls back to the
// <name>-report.md convention when the spec declares no output file.
func artifactNameFromSpec(spec phasespec.PhaseSpec) string {
	if len(spec.Outputs.Files) > 0 && spec.Outputs.Files[0] != "" {
		return filepath.Base(spec.Outputs.Files[0])
	}
	return spec.Name + "-report.md"
}

// kindFromArtifact maps a filename extension to a deliverable Kind. Note this is
// orthogonal to PhaseSpec.Kind (llm|native|command, which describes the runner,
// not the deliverable shape).
func kindFromArtifact(name string) Kind {
	if strings.EqualFold(filepath.Ext(name), ".json") {
		return KindJSON
	}
	return KindMarkdown
}

// sectionsFromClassify turns classify.require_sections into tolerant Sections.
// Each entry yields a "## "-prefixed Canonical heading; Accepted carries both the
// prefixed heading and the bare token so a report written with either form
// satisfies the section (Section.Present is a substring match). An author who
// already prefixed with "#" is not double-prefixed.
func sectionsFromClassify(spec phasespec.PhaseSpec) []Section {
	if spec.Classify == nil || len(spec.Classify.RequireSections) == 0 {
		return nil
	}
	sections := make([]Section, 0, len(spec.Classify.RequireSections))
	for _, raw := range spec.Classify.RequireSections {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		if strings.HasPrefix(s, "#") {
			sections = append(sections, Section{Canonical: s, Accepted: []string{s}})
			continue
		}
		canonical := "## " + s
		sections = append(sections, Section{Canonical: canonical, Accepted: []string{canonical, s}})
	}
	return sections
}

// verdictsFromSpec returns the standard verdict vocabulary ONLY when the spec
// opts in: an Evaluate-archetype phase that declares classify.verdict_on_pass.
// Any other phase returns nil — never auto-attaching a verdict gate the agent
// does not actually emit (the cycle-192 false-FAIL class).
func verdictsFromSpec(spec phasespec.PhaseSpec) []string {
	if spec.RoleOrDefault() != phasespec.RoleEvaluate {
		return nil
	}
	if spec.Classify == nil || spec.Classify.VerdictOnPass == "" {
		return nil
	}
	return []string{"PASS", "FAIL", "WARN", "SKIPPED"}
}

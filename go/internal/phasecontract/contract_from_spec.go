package phasecontract

import (
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// FromSpec derives a well-formedness-only Contract from a PhaseSpec, for phases the built-in map lacks. See ADR-0035.
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
		// Only meaningful for a phase that emits verdicts.
		RequireFailureContext: len(verdicts) > 0 && spec.Classify != nil && spec.Classify.RequireFailureContext,
		AgentOwedFiles:        spec.Outputs.AgentOwed,
		Effects:               spec.Effects,
	}
}

// overlayDeclared copies the registry-only fields onto a built-in contract, which never declares them itself.
func overlayDeclared(c Contract, spec phasespec.PhaseSpec) Contract {
	c.AgentOwedFiles = spec.Outputs.AgentOwed
	c.Effects = spec.Effects
	return c
}

// SynthesizesContract reports whether a spec yields a derived contract: an llm phase, or one declaring outputs.files.
func SynthesizesContract(spec phasespec.PhaseSpec) bool {
	if spec.KindOrDefault() == "llm" {
		return true
	}
	return len(spec.Outputs.Files) > 0 && spec.Outputs.Files[0] != ""
}

// artifactNameFromSpec returns the basename of outputs.files[0], else the <name>-report.md convention.
func artifactNameFromSpec(spec phasespec.PhaseSpec) string {
	if len(spec.Outputs.Files) > 0 && spec.Outputs.Files[0] != "" {
		return filepath.Base(spec.Outputs.Files[0])
	}
	return spec.Name + "-report.md"
}

// kindFromArtifact maps a filename extension to a deliverable Kind; PhaseSpec.Kind describes the runner instead.
func kindFromArtifact(name string) Kind {
	if strings.EqualFold(filepath.Ext(name), ".json") {
		return KindJSON
	}
	return KindMarkdown
}

// sectionsFromClassify accepts both the "## "-prefixed heading and the bare token for each required section.
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

// verdictsFromSpec returns the verdict vocabulary only for an evaluate phase that declares classify.verdict_on_pass,
// so a phase is never gated on a verdict it does not emit.
func verdictsFromSpec(spec phasespec.PhaseSpec) []string {
	if spec.RoleOrDefault() != phasespec.RoleEvaluate {
		return nil
	}
	if spec.Classify == nil || spec.Classify.VerdictOnPass == "" {
		return nil
	}
	return []string{"PASS", "FAIL", "WARN", "SKIPPED"}
}

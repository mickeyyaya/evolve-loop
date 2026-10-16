package phasecontract

import (
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func FromSpec(spec phasespec.PhaseSpec) Contract {
	verdicts := verdictsFromSpec(spec)
	return Contract{
		Phase:                 spec.Name,
		AgentName:             spec.AgentName(),
		ArtifactName:          artifactNameFromSpec(spec),
		Kind:                  kindFromArtifact(artifactNameFromSpec(spec)),
		Sections:              sectionsFromClassify(spec),
		Verdicts:              verdicts,
		RequiredKeys:          nil,
		WriteTarget:           TargetWorkspace,
		RequireFailureContext: len(verdicts) > 0 && spec.Classify != nil && spec.Classify.RequireFailureContext,
		AgentOwedFiles:        spec.Outputs.AgentOwed,
		Effects:               spec.Effects,
		DerivedFrom:           spec.Outputs.DerivedFrom,
		Grammars:              grammarsFromSpec(spec),
	}
}

func grammarsFromSpec(spec phasespec.PhaseSpec) []string {
	if spec.Classify == nil {
		return nil
	}
	return spec.Classify.Grammars
}

func overlayDeclared(c Contract, spec phasespec.PhaseSpec) Contract {
	c.AgentOwedFiles = spec.Outputs.AgentOwed
	c.Effects = spec.Effects
	c.DerivedFrom = spec.Outputs.DerivedFrom
	return c
}

func SynthesizesContract(spec phasespec.PhaseSpec) bool {
	if spec.KindOrDefault() == "llm" {
		return true
	}
	return len(spec.Outputs.Files) > 0 && spec.Outputs.Files[0] != ""
}

func artifactNameFromSpec(spec phasespec.PhaseSpec) string {
	if len(spec.Outputs.Files) > 0 && spec.Outputs.Files[0] != "" {
		return filepath.Base(spec.Outputs.Files[0])
	}
	return spec.Name + "-report.md"
}

func kindFromArtifact(name string) Kind {
	if strings.EqualFold(filepath.Ext(name), ".json") {
		return KindJSON
	}
	return KindMarkdown
}

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

func verdictsFromSpec(spec phasespec.PhaseSpec) []string {
	if spec.RoleOrDefault() != phasespec.RoleEvaluate {
		return nil
	}
	if spec.Classify == nil || spec.Classify.VerdictOnPass == "" {
		return nil
	}
	return []string{"PASS", "FAIL", "WARN", "SKIPPED"}
}

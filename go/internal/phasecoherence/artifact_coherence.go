package phasecoherence

import (
	"fmt"
	"path"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// CheckArtifactNames reports each persona whose first output-format .md token disagrees with
// its profile's output_artifact; it errors only on configuration or I/O failure.
func CheckArtifactNames(opts Options) ([]Violation, error) {
	var violations []Violation
	err := walkPersonas(opts, personaVisitor{
		paired: func(name string, profile profiles.Profile) error {
			v, err := checkArtifactName(opts, name, profile)
			if v != nil {
				violations = append(violations, *v)
			}
			return err
		},
	})
	if err != nil {
		return nil, err
	}
	return violations, nil
}

func checkArtifactName(opts Options, name string, profile profiles.Profile) (*Violation, error) {
	fm, err := personaFrontmatter(opts, name)
	if err != nil || fm == nil {
		return nil, err
	}
	outputFormatStr, ok := fm["output-format"].(string)
	if !ok {
		return nil, nil
	}
	declared := path.Base(firstMdToken(outputFormatStr))
	if declared == "." {
		return nil, nil
	}
	return artifactNameViolation(name, declared, profile.OutputArtifact), nil
}

func artifactNameViolation(name, declared, outputArtifact string) *Violation {
	if outputArtifact == "" {
		return &Violation{
			Persona:  name,
			Kind:     "mismatch",
			Severity: SeverityWarn,
			Message:  fmt.Sprintf("mismatch: persona declares output artifact %q but profile has no output_artifact field", declared),
		}
	}

	if path.Ext(outputArtifact) != ".md" {
		return nil
	}

	profileArtifact := path.Base(outputArtifact)
	if declared == profileArtifact {
		return nil
	}
	return &Violation{
		Persona:  name,
		Kind:     "mismatch",
		Severity: SeverityWarn,
		Message:  fmt.Sprintf("mismatch: persona declares output artifact %q but profile specifies %q", declared, profileArtifact),
	}
}

func firstMdToken(s string) string {
	fields := strings.Fields(s)
	for _, f := range fields {
		trimmed := strings.Trim(f, `"'(),`)
		if strings.HasSuffix(trimmed, ".md") {
			return trimmed
		}
	}
	return ""
}

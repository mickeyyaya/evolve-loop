package phasecoherence

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

func CheckArtifactNames(opts Options) ([]Violation, error) {
	if opts.AgentsFS == nil {
		return nil, errors.New("missing AgentsFS")
	}
	if opts.ProfilesFS == nil {
		return nil, errors.New("missing ProfilesFS")
	}

	entries, err := fs.ReadDir(opts.AgentsFS, "agents")
	if err != nil {
		return nil, fmt.Errorf("read agents: %w", err)
	}

	loader := profiles.NewFromFS(opts.ProfilesFS)
	var violations []Violation

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		n := entry.Name()
		if !strings.HasSuffix(n, ".md") || !strings.HasPrefix(n, "evolve-") {
			continue
		}
		name := strings.TrimPrefix(strings.TrimSuffix(n, ".md"), "evolve-")

		// Check if profile exists
		profile, err := loader.Get(name)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}

		// Read persona file
		var personaBytes []byte
		if overridePath, ok := opts.Overrides[name]; ok {
			personaBytes, err = os.ReadFile(overridePath)
			if err != nil {
				return nil, err
			}
		} else {
			personaBytes, err = fs.ReadFile(opts.AgentsFS, "agents/evolve-"+name+".md")
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				return nil, err
			}
		}

		// Parse persona frontmatter
		fm, _, err := prompts.ParseFrontmatter(string(personaBytes))
		if err != nil {
			return nil, err
		}
		if fm == nil {
			continue
		}

		outputFormatVal, ok := fm["output-format"]
		if !ok {
			// No output-format: line -> skip
			continue
		}

		outputFormatStr, ok := outputFormatVal.(string)
		if !ok {
			continue
		}

		declared := firstMdToken(outputFormatStr)
		if declared == "" {
			continue
		}

		profileArtifact := path.Base(profile.OutputArtifact)

		if profile.OutputArtifact == "" {
			violations = append(violations, Violation{
				Persona:  name,
				Kind:     "mismatch",
				Severity: "WARN",
				Message:  fmt.Sprintf("mismatch: persona declares output artifact %q but profile has no output_artifact field", declared),
			})
			continue
		}

		if declared != profileArtifact {
			violations = append(violations, Violation{
				Persona:  name,
				Kind:     "mismatch",
				Severity: "WARN",
				Message:  fmt.Sprintf("mismatch: persona declares output artifact %q but profile specifies %q", declared, profileArtifact),
			})
		}
	}

	return violations, nil
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

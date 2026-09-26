package phasecoherence

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

// CheckArtifactNames reports each persona whose first output-format .md token disagrees with
// its profile's output_artifact; it errors only on configuration or I/O failure.
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

		profile, err := loader.Get(name)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}

		persona, err := personaContents(opts, name)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && opts.Overrides[name] == "" {
				continue
			}
			return nil, err
		}

		fm, _, err := prompts.ParseFrontmatter(persona)
		if err != nil {
			return nil, err
		}
		if fm == nil {
			continue
		}

		outputFormatVal, ok := fm["output-format"]
		if !ok {
			continue
		}

		outputFormatStr, ok := outputFormatVal.(string)
		if !ok {
			continue
		}

		// Both sides compare by basename: a persona token may be dir-qualified
		// ("learn/reflector-synthesis.md").
		declared := path.Base(firstMdToken(outputFormatStr))
		if declared == "." {
			continue
		}

		if profile.OutputArtifact == "" {
			violations = append(violations, Violation{
				Persona:  name,
				Kind:     "mismatch",
				Severity: "WARN",
				Message:  fmt.Sprintf("mismatch: persona declares output artifact %q but profile has no output_artifact field", declared),
			})
			continue
		}

		profileArtifact := path.Base(profile.OutputArtifact)

		// Beside a non-.md deliverable (memo's carryover-todos.json), the persona's
		// first .md token names a secondary artifact, so there is nothing to compare.
		if path.Ext(profile.OutputArtifact) != ".md" {
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

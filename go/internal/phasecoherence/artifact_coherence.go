package phasecoherence

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/profiles"
	"github.com/mickeyyaya/evolveloop/go/internal/prompts"
)

// CheckArtifactNames verifies that each persona's declared output-artifact
// (its first .md token) matches the output_artifact named in the paired
// profile. It returns a "mismatch" Violation per disagreement and a non-nil
// error only on configuration or I/O failure.
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
		persona, err := personaContents(opts, name)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && opts.Overrides[name] == "" {
				continue
			}
			return nil, err
		}

		// Parse persona frontmatter
		fm, _, err := prompts.ParseFrontmatter(persona)
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

		// Compare by basename on BOTH sides: persona tokens may be
		// dir-qualified (reflector's "learn/reflector-synthesis.md") while
		// the profile side is already path.Base'd.
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

		// A non-.md contract deliverable (memo → carryover-todos.json) can
		// never match the persona's first .md token — that token is a
		// legitimate SECONDARY artifact, not the contract one. Skip.
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

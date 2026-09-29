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
		v, err := checkArtifactNameEntry(opts, loader, entry)
		if err != nil {
			return nil, err
		}
		if v != nil {
			violations = append(violations, *v)
		}
	}

	return violations, nil
}

func checkArtifactNameEntry(opts Options, loader *profiles.Loader, entry fs.DirEntry) (*Violation, error) {
	if entry.IsDir() {
		return nil, nil
	}
	n := entry.Name()
	if !strings.HasSuffix(n, ".md") || !strings.HasPrefix(n, "evolve-") {
		return nil, nil
	}
	name := strings.TrimPrefix(strings.TrimSuffix(n, ".md"), "evolve-")

	profile, err := loader.Get(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	persona, err := personaContents(opts, name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && opts.Overrides[name] == "" {
			return nil, nil
		}
		return nil, err
	}

	fm, _, err := prompts.ParseFrontmatter(persona)
	if err != nil {
		return nil, err
	}
	if fm == nil {
		return nil, nil
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
			Severity: "WARN",
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
		Severity: "WARN",
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

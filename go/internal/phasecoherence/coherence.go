package phasecoherence

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

type Violation struct {
	Persona  string // base name, e.g. "builder" (evolve- prefix stripped)
	Kind     string // "disallowed" | "undeclared" (tools checks)
	Severity string // "WARN" for both drift directions
	Message  string // eval vocabulary: contradiction|mismatch|disallowed|undeclared
}

type Options struct {
	AgentsFS   fs.FS             // root CONTAINING agents/ (prompts.Loader layout)
	ProfilesFS fs.FS             // profiles dir root: <name>.json at top level (profiles.Loader layout)
	Overrides  map[string]string // persona name → OS file path substituting agents/evolve-<name>.md
}

func Check(opts Options) ([]Violation, error) {
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

		if len(profile.AllowedTools) == 0 {
			// profile without allowed_tools -> no constraint -> skip.
			continue
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

		toolsVal, ok := fm["tools"]
		if !ok {
			// No tools: line -> skip
			continue
		}

		// toolsVal parsed by parseValue is a []string if it was an array
		toolsSlice, ok := toolsVal.([]string)
		if !ok {
			continue
		}

		violations = append(violations, checkToolsCoherence(name, toolsSlice, profile.AllowedTools)...)
	}

	return violations, nil
}

func normalizeToolName(name string) string {
	if idx := strings.Index(name, "("); idx != -1 {
		return strings.TrimSpace(name[:idx])
	}
	return strings.TrimSpace(name)
}

func checkToolsCoherence(name string, personaTools, allowedTools []string) []Violation {
	var vs []Violation

	// Normalize allowedTools
	allowedSet := make(map[string]bool)
	for _, t := range allowedTools {
		allowedSet[normalizeToolName(t)] = true
	}

	// Normalize personaTools
	personaSet := make(map[string]bool)
	for _, t := range personaTools {
		personaSet[normalizeToolName(t)] = true
	}

	// 1. Check for disallowed tools (persona has, profile allowedTools doesn't have)
	for _, pt := range personaTools {
		normPt := normalizeToolName(pt)
		if !allowedSet[normPt] {
			vs = append(vs, Violation{
				Persona:  name,
				Kind:     "disallowed",
				Severity: "WARN",
				Message:  fmt.Sprintf("contradiction: tool %q is disallowed by profile", pt),
			})
		}
	}

	// 2. Check for undeclared tools (profile allowedTools has, persona doesn't have)
	reportedUndeclared := make(map[string]bool)
	for _, at := range allowedTools {
		normAt := normalizeToolName(at)
		if !personaSet[normAt] && !reportedUndeclared[normAt] {
			reportedUndeclared[normAt] = true
			vs = append(vs, Violation{
				Persona:  name,
				Kind:     "undeclared",
				Severity: "WARN",
				Message:  fmt.Sprintf("mismatch: allowed tool %q is undeclared in persona", at),
			})
		}
	}

	return vs
}

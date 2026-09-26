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

// Violation is one drift finding from Check, CheckArtifactNames or CheckProvenance.
type Violation struct {
	Persona  string // base name without the evolve- prefix, e.g. "builder"; empty for provenance findings
	Kind     string // "unpaired" | "disallowed" | "undeclared" | "mismatch" | "missing-provenance" | "provenance-mismatch"
	Severity string // "WARN" for persona drift and a missing header; "error" for a provenance mismatch
	Message  string // persona drift uses the eval vocabulary: contradiction|mismatch|disallowed|undeclared
}

// Options names the persona and profile roots a check reads.
type Options struct {
	AgentsFS   fs.FS             // root CONTAINING agents/ (prompts.Loader layout)
	ProfilesFS fs.FS             // profiles dir root: <name>.json at top level (profiles.Loader layout)
	Overrides  map[string]string // persona name → OS file path substituting agents/evolve-<name>.md
}

// Check reports each evolve-<name>.md persona that lacks a profile or whose tools disagree
// with the profile's allowed_tools; it errors only on configuration or I/O failure.
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

		profile, err := loader.Get(name)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// -reference personas are documentation and dispatch: none personas run
				// outside the profile system; every other unpaired persona WARNs.
				if !strings.HasSuffix(name, "-reference") && !dispatchNone(opts, name) {
					violations = append(violations, Violation{
						Persona:  name,
						Kind:     "unpaired",
						Severity: "WARN",
						Message:  "mismatch: persona agents/evolve-" + name + ".md has no profile .evolve/profiles/" + name + ".json — undispatchable (dies exit=10 at launch if routed)",
					})
				}
				continue
			}
			return nil, err
		}

		if len(profile.AllowedTools) == 0 {
			// An absent or empty allowed_tools means no constraint, not "nothing allowed".
			continue
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

		toolsVal, ok := fm["tools"]
		if !ok {
			continue
		}

		toolsSlice, ok := toolsVal.([]string)
		if !ok {
			continue
		}

		violations = append(violations, checkToolsCoherence(name, toolsSlice, profile.AllowedTools)...)
	}

	return violations, nil
}

// dispatchNone reports whether the persona opts out of profile pairing with `dispatch: none`.
// An unreadable or unparsable persona returns false, so the unpaired WARN still fires.
func dispatchNone(opts Options, name string) bool {
	raw, err := personaContents(opts, name)
	if err != nil {
		return false
	}
	fm, _, err := prompts.ParseFrontmatter(raw)
	if err != nil || fm == nil {
		return false
	}
	v, ok := fm["dispatch"].(string)
	return ok && strings.TrimSpace(v) == "none"
}

// personaContents reads agents/evolve-<name>.md, honoring Overrides.
func personaContents(opts Options, name string) (string, error) {
	if overridePath, ok := opts.Overrides[name]; ok {
		b, err := os.ReadFile(overridePath)
		return string(b), err
	}
	b, err := fs.ReadFile(opts.AgentsFS, "agents/evolve-"+name+".md")
	return string(b), err
}

func normalizeToolName(name string) string {
	if idx := strings.Index(name, "("); idx != -1 {
		return strings.TrimSpace(name[:idx])
	}
	return strings.TrimSpace(name)
}

func checkToolsCoherence(name string, personaTools, allowedTools []string) []Violation {
	var vs []Violation

	allowedSet := make(map[string]bool)
	for _, t := range allowedTools {
		allowedSet[normalizeToolName(t)] = true
	}

	personaSet := make(map[string]bool)
	for _, t := range personaTools {
		personaSet[normalizeToolName(t)] = true
	}

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

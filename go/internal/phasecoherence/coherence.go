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

// Violation is one persona/profile coherence drift finding, carrying the
// affected persona, the drift Kind, a Severity, and an eval-vocabulary Message.
type Violation struct {
	Persona  string // base name, e.g. "builder" (evolve- prefix stripped)
	Kind     string // "disallowed" | "undeclared" (tools checks) | "unpaired" (missing profile)
	Severity string // "WARN" for both drift directions
	Message  string // eval vocabulary: contradiction|mismatch|disallowed|undeclared
}

// Options configures a coherence check: the filesystem containing agents/,
// the profiles filesystem, and optional per-persona path overrides.
type Options struct {
	AgentsFS   fs.FS             // root CONTAINING agents/ (prompts.Loader layout)
	ProfilesFS fs.FS             // profiles dir root: <name>.json at top level (profiles.Loader layout)
	Overrides  map[string]string // persona name → OS file path substituting agents/evolve-<name>.md
}

// Check verifies that each evolve-<name>.md persona is paired with a profile
// and that the persona's declared tools agree with the profile's allowed_tools.
// It returns one Violation per drift (unpaired profile, disallowed tool, or
// undeclared tool) and a non-nil error only on configuration or I/O failure.
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
				// "-reference" personas are documentation (auditor-reference
				// etc.), never dispatched — no profile expected. Likewise a
				// persona whose frontmatter declares `dispatch: none`
				// (operator: monitoring persona driven outside the
				// profile/subagent system) is intentionally unpaired.
				// Everything else unpaired is a visibility WARN, not a silent
				// skip: cycle-270's debugger died at launch (exit=10) because
				// its persona existed and its profile didn't, and nothing said
				// so until the route fired (inbox
				// dispatchable-agent-profile-completeness).
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
			// profile without allowed_tools -> no constraint -> skip.
			continue
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

// dispatchNone reports whether the persona's frontmatter opts out of profile
// pairing with `dispatch: none`. Best-effort: unreadable/unparsable personas
// return false so the unpaired WARN still fires.
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

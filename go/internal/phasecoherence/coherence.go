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
	Persona  string
	Kind     string
	Severity string
	Message  string
}

const (
	SeverityWarn  = "WARN"
	SeverityError = "ERROR"
)

type Options struct {
	AgentsFS   fs.FS
	ProfilesFS fs.FS
	Overrides  map[string]string
}

func Check(opts Options) ([]Violation, error) {
	var violations []Violation
	err := walkPersonas(opts, personaVisitor{
		unpaired: func(name string) {
			violations = append(violations, unpairedViolations(opts, name)...)
		},
		paired: func(name string, profile profiles.Profile) error {
			vs, err := checkPersonaTools(opts, name, profile)
			violations = append(violations, vs...)
			return err
		},
	})
	if err != nil {
		return nil, err
	}
	return violations, nil
}

type personaVisitor struct {
	unpaired func(name string)
	paired   func(name string, profile profiles.Profile) error
}

func walkPersonas(opts Options, visit personaVisitor) error {
	if opts.AgentsFS == nil {
		return errors.New("missing AgentsFS")
	}
	if opts.ProfilesFS == nil {
		return errors.New("missing ProfilesFS")
	}

	entries, err := fs.ReadDir(opts.AgentsFS, "agents")
	if err != nil {
		return fmt.Errorf("read agents: %w", err)
	}

	loader := profiles.NewFromFS(opts.ProfilesFS)
	for _, entry := range entries {
		name, ok := personaName(entry)
		if !ok {
			continue
		}
		profile, err := loader.Get(name)
		if errors.Is(err, fs.ErrNotExist) {
			if visit.unpaired != nil {
				visit.unpaired(name)
			}
			continue
		}
		if err != nil {
			return err
		}
		if err := visit.paired(name, profile); err != nil {
			return err
		}
	}
	return nil
}

func personaName(entry fs.DirEntry) (string, bool) {
	n := entry.Name()
	if entry.IsDir() || !strings.HasSuffix(n, ".md") || !strings.HasPrefix(n, "evolve-") {
		return "", false
	}
	return strings.TrimPrefix(strings.TrimSuffix(n, ".md"), "evolve-"), true
}

func personaFrontmatter(opts Options, name string) (map[string]any, error) {
	persona, err := personaContents(opts, name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && opts.Overrides[name] == "" {
			return nil, nil
		}
		return nil, err
	}
	fm, _, err := prompts.ParseFrontmatter(persona)
	return fm, err
}

func checkPersonaTools(opts Options, name string, profile profiles.Profile) ([]Violation, error) {
	if len(profile.AllowedTools) == 0 {
		return nil, nil
	}
	fm, err := personaFrontmatter(opts, name)
	if err != nil || fm == nil {
		return nil, err
	}
	toolsSlice, ok := fm["tools"].([]string)
	if !ok {
		return nil, nil
	}
	return checkToolsCoherence(name, toolsSlice, profile.AllowedTools), nil
}

func unpairedViolations(opts Options, name string) []Violation {
	if strings.HasSuffix(name, "-reference") || dispatchNone(opts, name) {
		return nil
	}
	return []Violation{{
		Persona:  name,
		Kind:     "unpaired",
		Severity: SeverityWarn,
		Message:  "mismatch: persona agents/evolve-" + name + ".md has no profile .evolve/profiles/" + name + ".json — undispatchable (dies exit=10 at launch if routed)",
	}}
}

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
				Severity: SeverityWarn,
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
				Severity: SeverityWarn,
				Message:  fmt.Sprintf("mismatch: allowed tool %q is undeclared in persona", at),
			})
		}
	}

	return vs
}

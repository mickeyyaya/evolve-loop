// codex.go is the third projection surface (after the phase-facts regions and the
// commands/<name>.md stubs): it renders Codex's plugin manifests from the
// canonical .claude-plugin/plugin.json so a Codex user installs the SAME 23-skill
// set from the SAME single source as Claude Code.
//
// Codex 0.142.2 ships a native plugin + marketplace system that is a near-clone
// of Claude Code's: `codex plugin marketplace add <owner/repo>` reads
// .agents/plugins/marketplace.json at the repo root, and for the plugin it points
// at (source.path ".") it reads .codex-plugin/plugin.json (with "skills":
// "./skills/"). Mirroring Claude's `/plugin marketplace add` + `/plugin install
// evo@evo`, the Codex flow is `codex plugin marketplace add mickeyyaya/evolve-loop`
// + `codex plugin add evo@evo`.
//
// Both manifests are GENERATED here, never hand-edited, so name/version/metadata
// cannot drift from the Claude manifest. `evolve skills check` (run in CI and the
// cycle audit) gates that drift, and versionbump bumps the Codex version in
// lockstep at release — its surgical regex replace preserves this file's exact
// MarshalIndent formatting, so "generate" and "release bump" stay byte-identical.
//
// source.path is "." (the repo root IS the plugin) rather than a vendored
// plugins/evo/ copy: that keeps the 23 canonical skills single-sourced under
// skills/ with zero duplication, at the cost of `marketplace add` git-fetching the
// whole repo.
package skillcheck

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	codexPluginManifestRel = ".codex-plugin/plugin.json"
	codexMarketplaceRel    = ".agents/plugins/marketplace.json"

	// Codex-specific projection constants — fields with no Claude-manifest
	// counterpart. Codex's plugin manifest points at the bundled skills dir and
	// carries an interface block for its plugin browser.
	codexSkillsDir        = "./skills/"
	codexDisplayName      = "Evolve Loop"
	codexShortDescription = "Self-evolving development pipeline with eval gating and continuous learning"
	codexCategory         = "Developer Tools"
	// Codex's .strict() marketplace schema accepts only ON_INSTALL or ON_USE for
	// authentication (NOT "NONE"); evo needs no auth, so ON_USE.
	codexInstallation   = "AVAILABLE"
	codexAuthentication = "ON_USE"
	codexLocalSource    = "local"
	codexSourcePath     = "."
)

type codexAuthor struct {
	Name string `json:"name"`
}

// claudePluginMeta is the subset of .claude-plugin/plugin.json the Codex
// manifests project from — the single source for every shared field.
type claudePluginMeta struct {
	Name        string      `json:"name"`
	Version     string      `json:"version"`
	Description string      `json:"description"`
	Author      codexAuthor `json:"author"`
	Homepage    string      `json:"homepage"`
	Repository  string      `json:"repository"`
	License     string      `json:"license"`
	Keywords    []string    `json:"keywords"`
}

type codexInterface struct {
	DisplayName      string `json:"displayName"`
	ShortDescription string `json:"shortDescription"`
	Category         string `json:"category"`
}

type codexPluginManifest struct {
	claudePluginMeta
	Skills    string         `json:"skills"`
	Interface codexInterface `json:"interface"`
}

type codexMarketplaceInterface struct {
	DisplayName string `json:"displayName"`
}

type codexMarketplaceSource struct {
	Source string `json:"source"`
	Path   string `json:"path"`
}

type codexMarketplacePolicy struct {
	Installation   string `json:"installation"`
	Authentication string `json:"authentication"`
}

type codexMarketplacePlugin struct {
	Name     string                 `json:"name"`
	Source   codexMarketplaceSource `json:"source"`
	Policy   codexMarketplacePolicy `json:"policy"`
	Category string                 `json:"category"`
}

type codexMarketplaceManifest struct {
	Name      string                    `json:"name"`
	Interface codexMarketplaceInterface `json:"interface"`
	Plugins   []codexMarketplacePlugin  `json:"plugins"`
}

// loadClaudePluginMeta reads the canonical Claude plugin manifest.
func loadClaudePluginMeta(projectRoot string) (claudePluginMeta, error) {
	var m claudePluginMeta
	raw, err := os.ReadFile(filepath.Join(projectRoot, ".claude-plugin", "plugin.json"))
	if err != nil {
		return m, fmt.Errorf("read .claude-plugin/plugin.json: %w", err)
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, fmt.Errorf("parse .claude-plugin/plugin.json: %w", err)
	}
	return m, nil
}

// renderCodexPluginManifest projects .codex-plugin/plugin.json. Deterministic:
// struct field order + a trailing newline give stable bytes for the drift diff.
func renderCodexPluginManifest(m claudePluginMeta) ([]byte, error) {
	return marshalManifest(codexPluginManifest{
		claudePluginMeta: m,
		Skills:           codexSkillsDir,
		Interface: codexInterface{
			DisplayName:      codexDisplayName,
			ShortDescription: codexShortDescription,
			Category:         codexCategory,
		},
	})
}

// renderCodexMarketplace projects .agents/plugins/marketplace.json — the
// repo-root marketplace `codex plugin marketplace add` reads.
func renderCodexMarketplace(m claudePluginMeta) ([]byte, error) {
	out := codexMarketplaceManifest{
		Name:      m.Name,
		Interface: codexMarketplaceInterface{DisplayName: codexDisplayName},
		Plugins: []codexMarketplacePlugin{{
			Name:     m.Name,
			Source:   codexMarketplaceSource{Source: codexLocalSource, Path: codexSourcePath},
			Policy:   codexMarketplacePolicy{Installation: codexInstallation, Authentication: codexAuthentication},
			Category: codexCategory,
		}},
	}
	return marshalManifest(out)
}

// marshalManifest gives the canonical 2-space-indented + trailing-newline form
// shared by both Codex manifests.
func marshalManifest(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// codexManifestDiffs renders the desired Codex manifests from the canonical
// Claude plugin.json and diffs each against disk. Pure: no writes, no prints.
func codexManifestDiffs(projectRoot string) ([]commandDiff, error) {
	meta, err := loadClaudePluginMeta(projectRoot)
	if err != nil {
		// A checkout without the canonical Claude manifest is not an evo plugin
		// repo — there is nothing to project. Tolerated-absent, never fatal.
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	specs := []struct {
		rel    string
		render func(claudePluginMeta) ([]byte, error)
	}{
		{codexPluginManifestRel, renderCodexPluginManifest},
		{codexMarketplaceRel, renderCodexMarketplace},
	}
	diffs := make([]commandDiff, 0, len(specs))
	for _, s := range specs {
		next, err := s.render(meta)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", s.rel, err)
		}
		path := filepath.Join(projectRoot, filepath.FromSlash(s.rel))
		cur, _ := os.ReadFile(path)
		diffs = append(diffs, commandDiff{
			rel:     s.rel,
			path:    path,
			next:    string(next),
			drifted: !bytes.Equal(cur, next),
		})
	}
	return diffs, nil
}

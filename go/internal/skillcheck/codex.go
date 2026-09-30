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

	codexSkillsDir        = "./skills/"
	codexDisplayName      = "Evolve Loop"
	codexShortDescription = "Self-evolving development pipeline with eval gating and continuous learning"
	codexCategory         = "Developer Tools"
	codexInstallation     = "AVAILABLE"
	codexAuthentication   = "ON_USE"
	codexLocalSource      = "local"
	codexSourcePath       = "."
)

type codexAuthor struct {
	Name string `json:"name"`
}

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

func marshalManifest(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func codexManifestDiffs(projectRoot string) ([]commandDiff, error) {
	meta, err := loadClaudePluginMeta(projectRoot)
	if err != nil {
		isNotAnEvoPluginRepo := errors.Is(err, os.ErrNotExist)
		if isNotAnEvoPluginRepo {
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

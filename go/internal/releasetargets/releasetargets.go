// Package releasetargets parses .goreleaser.yml — the single source of
// truth for the release build matrix — into the target list and
// asset-naming rules the release-verify-binaries gate needs.
package releasetargets

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/yamlcodec"
)

type Target struct {
	OS   string
	Arch string
}

func (t Target) String() string { return t.OS + "_" + t.Arch }

type Config struct {
	Targets             []Target
	ArchiveNameTemplate string
	ArchiveFormat       string
	ChecksumsName       string
	RepoOwner           string
	RepoName            string
}

type goreleaserDoc struct {
	Builds []struct {
		Targets []string `yaml:"targets"`
	} `yaml:"builds"`
	UniversalBinaries []universalBinary `yaml:"universal_binaries"`
	Archives          []struct {
		NameTemplate string   `yaml:"name_template"`
		Formats      []string `yaml:"formats"`
	} `yaml:"archives"`
	Checksum struct {
		NameTemplate string `yaml:"name_template"`
	} `yaml:"checksum"`
	Release struct {
		GitHub struct {
			Owner string `yaml:"owner"`
			Name  string `yaml:"name"`
		} `yaml:"github"`
	} `yaml:"release"`
}

func ParseConfig(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read goreleaser config %s: %w", path, err)
	}
	var doc goreleaserDoc
	if err := yamlcodec.Unmarshal(raw, &doc); err != nil {
		return Config{}, fmt.Errorf("parse goreleaser config %s: %w", path, err)
	}

	cfg := Config{
		ChecksumsName: resolveChecksumsName(doc),
		RepoOwner:     doc.Release.GitHub.Owner,
		RepoName:      doc.Release.GitHub.Name,
	}
	if len(doc.Archives) > 0 {
		cfg.ArchiveNameTemplate = doc.Archives[0].NameTemplate
		if len(doc.Archives[0].Formats) > 0 {
			cfg.ArchiveFormat = doc.Archives[0].Formats[0]
		}
	}

	targets, err := collectBuildTargets(doc)
	if err != nil {
		return Config{}, err
	}
	cfg.Targets = appendUniversalDarwin(targets, doc)
	if len(cfg.Targets) == 0 {
		return Config{}, fmt.Errorf("goreleaser config %s declares zero build targets", path)
	}
	return cfg, nil
}

func resolveChecksumsName(doc goreleaserDoc) string {
	if doc.Checksum.NameTemplate != "" {
		return doc.Checksum.NameTemplate
	}
	return "checksums.txt"
}

func collectBuildTargets(doc goreleaserDoc) ([]Target, error) {
	var targets []Target
	seen := map[string]bool{}
	for _, b := range doc.Builds {
		for _, tgt := range b.Targets {
			goos, arch, ok := splitTarget(tgt)
			if !ok {
				return nil, fmt.Errorf("malformed target %q (want <os>_<arch>)", tgt)
			}
			key := goos + "_" + arch
			if seen[key] {
				continue
			}
			seen[key] = true
			targets = append(targets, Target{OS: goos, Arch: arch})
		}
	}
	return targets, nil
}

func appendUniversalDarwin(targets []Target, doc goreleaserDoc) []Target {
	if len(doc.UniversalBinaries) == 0 {
		return targets
	}
	if !hasTarget(targets, "darwin", "all") {
		targets = append(targets, Target{OS: "darwin", Arch: "all"})
	}
	if anyReplace(doc.UniversalBinaries) {
		targets = dropPerArchDarwin(targets)
	}
	return targets
}

func hasTarget(targets []Target, goos, arch string) bool {
	for _, t := range targets {
		if t.OS == goos && t.Arch == arch {
			return true
		}
	}
	return false
}

type universalBinary struct {
	ID      string `yaml:"id"`
	Replace bool   `yaml:"replace"`
}

func anyReplace(unis []universalBinary) bool {
	for _, u := range unis {
		if u.Replace {
			return true
		}
	}
	return false
}

func dropPerArchDarwin(targets []Target) []Target {
	kept := targets[:0:0]
	for _, t := range targets {
		if t.OS == "darwin" && t.Arch != "all" {
			continue
		}
		kept = append(kept, t)
	}
	return kept
}

func splitTarget(s string) (goos, arch string, ok bool) {
	i := strings.IndexByte(s, '_')
	if i <= 0 || i >= len(s)-1 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

func (c Config) AssetName(t Target) (string, error) {
	if c.ArchiveNameTemplate == "" {
		return "", fmt.Errorf("goreleaser config has no archives[].name_template")
	}
	if c.ArchiveFormat == "" {
		return "", fmt.Errorf("goreleaser config has no archives[].formats")
	}
	tmpl, err := template.New("archive").Parse(c.ArchiveNameTemplate)
	if err != nil {
		return "", fmt.Errorf("parse name_template %q: %w", c.ArchiveNameTemplate, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct{ Os, Arch string }{t.OS, t.Arch}); err != nil {
		return "", fmt.Errorf("render name_template for %s: %w", t, err)
	}
	return buf.String() + "." + c.ArchiveFormat, nil
}

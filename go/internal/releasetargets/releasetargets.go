// Package releasetargets parses .goreleaser.yml — the single source of truth for
// the release build matrix — into the target list and asset-naming rules the
// release-verify-binaries gate needs.
//
// Why parse goreleaser rather than re-list the targets in Go: goreleaser must
// carry its own inline builds[].targets (it cannot read external config for
// them), so it is the only list that genuinely must exist. Deriving the expected
// release assets from it — instead of duplicating the list — means adding a
// target to goreleaser automatically makes the gate require it published, with
// no second list to keep in sync.
package releasetargets

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// Target is one (OS, Arch) pair the release publishes a prebuilt binary for,
// e.g. {OS: "linux", Arch: "ppc64le"}.
type Target struct {
	OS   string
	Arch string
}

// String renders the canonical goreleaser target form, "<os>_<arch>".
func (t Target) String() string { return t.OS + "_" + t.Arch }

// Config is the subset of .goreleaser.yml the release gate cares about.
type Config struct {
	// Targets is builds[].targets across all build blocks, de-duplicated.
	Targets []Target
	// ArchiveNameTemplate is archives[].name_template (the Go-template body
	// goreleaser renders per target, e.g. "evolve_{{ .Os }}_{{ .Arch }}").
	ArchiveNameTemplate string
	// ArchiveFormat is the first archives[].formats entry, e.g. "tar.gz".
	ArchiveFormat string
	// ChecksumsName is checksum.name_template, e.g. "checksums.txt".
	ChecksumsName string
	// RepoOwner and RepoName are release.github.owner/name — the repository the
	// assets are published to. Read from the same SSOT so the gate needs no
	// hardcoded repo identity.
	RepoOwner string
	RepoName  string
}

// goreleaserDoc mirrors only the fields ParseConfig reads.
type goreleaserDoc struct {
	Builds []struct {
		Targets []string `yaml:"targets"`
	} `yaml:"builds"`
	Archives []struct {
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

// ParseConfig reads the goreleaser config at path. It errors if the file is
// unreadable, malformed, or declares zero build targets.
func ParseConfig(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read goreleaser config %s: %w", path, err)
	}
	var doc goreleaserDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return Config{}, fmt.Errorf("parse goreleaser config %s: %w", path, err)
	}

	cfg := Config{
		ChecksumsName: doc.Checksum.NameTemplate,
		RepoOwner:     doc.Release.GitHub.Owner,
		RepoName:      doc.Release.GitHub.Name,
	}
	// goreleaser defaults checksum.name_template to "checksums.txt" when omitted,
	// so the published asset is named checksums.txt regardless — default to it
	// here too rather than spuriously reporting the checksums row missing.
	if cfg.ChecksumsName == "" {
		cfg.ChecksumsName = "checksums.txt"
	}
	if len(doc.Archives) > 0 {
		cfg.ArchiveNameTemplate = doc.Archives[0].NameTemplate
		if len(doc.Archives[0].Formats) > 0 {
			cfg.ArchiveFormat = doc.Archives[0].Formats[0]
		}
	}

	seen := map[string]bool{}
	for _, b := range doc.Builds {
		for _, tgt := range b.Targets {
			goos, arch, ok := splitTarget(tgt)
			if !ok {
				return Config{}, fmt.Errorf("malformed target %q (want <os>_<arch>)", tgt)
			}
			key := goos + "_" + arch
			if seen[key] {
				continue
			}
			seen[key] = true
			cfg.Targets = append(cfg.Targets, Target{OS: goos, Arch: arch})
		}
	}
	if len(cfg.Targets) == 0 {
		return Config{}, fmt.Errorf("goreleaser config %s declares zero build targets", path)
	}
	return cfg, nil
}

// splitTarget splits a goreleaser target on the FIRST underscore so that arches
// containing no underscore of their own (ppc64le, riscv64, s390x) keep their
// full name: "linux_ppc64le" → ("linux", "ppc64le"). Named goos (not os) to
// avoid shadowing the imported os package.
func splitTarget(s string) (goos, arch string, ok bool) {
	i := strings.IndexByte(s, '_')
	if i <= 0 || i >= len(s)-1 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

// AssetName renders the published archive name for t, matching what goreleaser
// produces: name_template rendered with the target's Os/Arch, plus the archive
// format suffix. Returns an error if the config lacks a name template/format.
func (c Config) AssetName(t Target) (string, error) {
	if c.ArchiveNameTemplate == "" {
		return "", fmt.Errorf("goreleaser config has no archives[].name_template")
	}
	if c.ArchiveFormat == "" {
		return "", fmt.Errorf("goreleaser config has no archives[].formats")
	}
	// Parsed per call (not cached in ParseConfig) so AssetName works on ANY
	// Config — including hand-constructed ones in tests — rather than silently
	// failing when archiveTmpl was never populated. The matrix is ~13 targets,
	// so the repeated parse of a tiny template is negligible.
	tmpl, err := template.New("archive").Parse(c.ArchiveNameTemplate)
	if err != nil {
		return "", fmt.Errorf("parse name_template %q: %w", c.ArchiveNameTemplate, err)
	}
	var buf bytes.Buffer
	// goreleaser exposes the target as .Os/.Arch in archive templates.
	if err := tmpl.Execute(&buf, struct{ Os, Arch string }{t.OS, t.Arch}); err != nil {
		return "", fmt.Errorf("render name_template for %s: %w", t, err)
	}
	return buf.String() + "." + c.ArchiveFormat, nil
}

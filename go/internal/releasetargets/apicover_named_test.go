package releasetargets

import (
	"strings"
	"testing"
)

// TestTarget_String names the Target type and its String method, pinning the
// canonical "<os>_<arch>" rendering used in verifier error rows.
func TestTarget_String(t *testing.T) {
	got := Target{OS: "linux", Arch: "arm64"}.String()
	if got != "linux_arm64" {
		t.Fatalf("Target.String() = %q, want linux_arm64", got)
	}
}

// TestConfig_AssetNameErrors names the Config type literally and exercises
// AssetName's error branches: a missing name template, a missing format, a
// malformed template (parse error), and a template referencing a field the
// target struct does not expose (render error).
func TestConfig_AssetNameErrors(t *testing.T) {
	tg := Target{OS: "darwin", Arch: "arm64"}

	cases := []struct {
		name string
		cfg  Config
		want string // substring expected in the error
	}{
		{"no template", Config{ArchiveFormat: "tar.gz"}, "name_template"},
		{"no format", Config{ArchiveNameTemplate: "evolve_{{ .Os }}_{{ .Arch }}"}, "formats"},
		{"bad parse", Config{ArchiveNameTemplate: "evolve_{{ .Os", ArchiveFormat: "tar.gz"}, "name_template"},
		{"bad render", Config{ArchiveNameTemplate: "evolve_{{ .Nope }}", ArchiveFormat: "tar.gz"}, "name_template"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := c.cfg.AssetName(tg); err == nil {
				t.Fatalf("AssetName: want error containing %q, got nil", c.want)
			} else if !strings.Contains(err.Error(), c.want) {
				t.Errorf("AssetName error = %q, want substring %q", err, c.want)
			}
		})
	}
}

// TestParseConfig_MalformedTargetNoUnderscore covers splitTarget's rejection of
// a target with no os/arch separator.
func TestParseConfig_MalformedTargetNoUnderscore(t *testing.T) {
	p := writeGoreleaser(t, `
version: 2
builds:
  - { id: evolve, targets: [ darwinamd64 ] }
archives:
  - { id: evolve, name_template: 'evolve_{{ .Os }}_{{ .Arch }}', formats: [ 'tar.gz' ] }
checksum: { name_template: 'checksums.txt' }
`)
	if _, err := ParseConfig(p); err == nil {
		t.Fatal("want error for target with no underscore")
	}
}

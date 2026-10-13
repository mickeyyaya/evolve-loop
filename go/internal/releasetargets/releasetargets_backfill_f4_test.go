package releasetargets

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func parseYAMLF4(t *testing.T, body string) Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".goreleaser.yml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	return cfg
}

func TestParseConfig_RepeatedTargetsAcrossBuildsAreListedOnce(t *testing.T) {
	cfg := parseYAMLF4(t, `
builds:
  - targets: [linux_amd64, darwin_arm64]
  - targets: [linux_amd64, windows_amd64, darwin_arm64]
`)

	want := []Target{{OS: "linux", Arch: "amd64"}, {OS: "darwin", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}}
	if !reflect.DeepEqual(cfg.Targets, want) {
		t.Fatalf("targets = %+v, want %+v", cfg.Targets, want)
	}
}

func TestParseConfig_UniversalBinaryAddsTheAllTargetOnlyWhenNoBuildAlreadyDeclaredIt(t *testing.T) {
	declared := parseYAMLF4(t, `
builds:
  - targets: [darwin_all, darwin_arm64]
universal_binaries:
  - id: evolve
`)
	if got := countTarget(declared.Targets, "darwin", "all"); got != 1 {
		t.Errorf("an already-declared darwin_all must not be duplicated: %d in %+v", got, declared.Targets)
	}
	if got := countTarget(declared.Targets, "darwin", "arm64"); got != 1 {
		t.Errorf("without replace the per-arch archive stays required: %+v", declared.Targets)
	}

	added := parseYAMLF4(t, `
builds:
  - targets: [darwin_arm64, linux_amd64]
universal_binaries:
  - id: evolve
    replace: true
`)
	if countTarget(added.Targets, "darwin", "all") != 1 || countTarget(added.Targets, "darwin", "arm64") != 0 || countTarget(added.Targets, "linux", "amd64") != 1 {
		t.Errorf("replace drops the per-arch darwin archives and adds the universal one: %+v", added.Targets)
	}
}

func countTarget(targets []Target, goos, arch string) int {
	n := 0
	for _, t := range targets {
		if t.OS == goos && t.Arch == arch {
			n++
		}
	}
	return n
}

func TestHasTarget_MatchesOnBothOSAndArch(t *testing.T) {
	targets := []Target{{OS: "darwin", Arch: "arm64"}, {OS: "linux", Arch: "amd64"}}
	cases := []struct {
		goos, arch string
		want       bool
	}{
		{"darwin", "arm64", true},
		{"linux", "amd64", true},
		{"darwin", "amd64", false},
		{"linux", "arm64", false},
		{"windows", "amd64", false},
	}
	for _, tc := range cases {
		if got := hasTarget(targets, tc.goos, tc.arch); got != tc.want {
			t.Errorf("hasTarget(%s,%s) = %v, want %v", tc.goos, tc.arch, got, tc.want)
		}
	}
	if hasTarget(nil, "darwin", "arm64") {
		t.Error("no targets, no match")
	}
}

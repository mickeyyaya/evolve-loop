package releasetargets

import (
	"os"
	"path/filepath"
	"testing"
)

func writeGoreleaser(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), ".goreleaser.yml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return p
}

func TestParseConfig_TargetsAndAssetNames(t *testing.T) {
	p := writeGoreleaser(t, `
version: 2
builds:
  - id: evolve
    targets:
      - darwin_amd64
      - linux_arm
      - linux_ppc64le
archives:
  - id: evolve
    name_template: 'evolve_{{ .Os }}_{{ .Arch }}'
    formats: [ 'tar.gz' ]
checksum:
  name_template: 'checksums.txt'
`)
	cfg, err := ParseConfig(p)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if len(cfg.Targets) != 3 {
		t.Fatalf("want 3 targets, got %d: %v", len(cfg.Targets), cfg.Targets)
	}
	if cfg.ChecksumsName != "checksums.txt" {
		t.Errorf("ChecksumsName = %q, want checksums.txt", cfg.ChecksumsName)
	}

	want := map[string]bool{
		"evolve_darwin_amd64.tar.gz":  true,
		"evolve_linux_arm.tar.gz":     true,
		"evolve_linux_ppc64le.tar.gz": true,
	}
	for _, tg := range cfg.Targets {
		name, err := cfg.AssetName(tg)
		if err != nil {
			t.Fatalf("AssetName(%v): %v", tg, err)
		}
		if !want[name] {
			t.Errorf("unexpected asset name %q for %v", name, tg)
		}
		delete(want, name)
	}
	if len(want) != 0 {
		t.Errorf("asset names never produced: %v", want)
	}
}

func TestParseConfig_UniversalBinary(t *testing.T) {
	p := writeGoreleaser(t, `
version: 2
builds:
  - id: evolve
    targets:
      - darwin_amd64
      - darwin_arm64
      - linux_amd64
universal_binaries:
  - id: evolve-universal
    ids: [ evolve ]
    replace: false
    name_template: 'evolve'
archives:
  - id: evolve
    name_template: 'evolve_{{ .Os }}_{{ .Arch }}'
    formats: [ 'tar.gz' ]
checksum:
  name_template: 'checksums.txt'
`)
	cfg, err := ParseConfig(p)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	have := map[string]bool{}
	for _, tg := range cfg.Targets {
		name, err := cfg.AssetName(tg)
		if err != nil {
			t.Fatalf("AssetName(%v): %v", tg, err)
		}
		have[name] = true
	}
	if !have["evolve_darwin_all.tar.gz"] {
		t.Errorf("universal_binaries block must surface evolve_darwin_all.tar.gz; got %v", have)
	}
	for _, core := range []string{"evolve_darwin_amd64.tar.gz", "evolve_darwin_arm64.tar.gz"} {
		if !have[core] {
			t.Errorf("per-arch archive %q must remain with replace:false; got %v", core, have)
		}
	}
}

func TestParseConfig_UniversalBinary_Replace(t *testing.T) {
	p := writeGoreleaser(t, `
version: 2
builds:
  - id: evolve
    targets: [ darwin_amd64, darwin_arm64, linux_amd64 ]
universal_binaries:
  - id: evolve-universal
    ids: [ evolve ]
    replace: true
    name_template: 'evolve'
archives:
  - { id: evolve, name_template: 'evolve_{{ .Os }}_{{ .Arch }}', formats: [ 'tar.gz' ] }
checksum: { name_template: 'checksums.txt' }
`)
	cfg, err := ParseConfig(p)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	have := map[string]bool{}
	for _, tg := range cfg.Targets {
		name, err := cfg.AssetName(tg)
		if err != nil {
			t.Fatalf("AssetName(%v): %v", tg, err)
		}
		have[name] = true
	}
	if !have["evolve_darwin_all.tar.gz"] {
		t.Errorf("replace:true must still surface the universal; got %v", have)
	}
	if !have["evolve_linux_amd64.tar.gz"] {
		t.Errorf("replace:true must keep non-darwin targets; got %v", have)
	}
	for _, dropped := range []string{"evolve_darwin_amd64.tar.gz", "evolve_darwin_arm64.tar.gz"} {
		if have[dropped] {
			t.Errorf("replace:true must DROP per-arch darwin %q (goreleaser stops publishing it); got %v", dropped, have)
		}
	}
}

func TestParseConfig_NoUniversalBinary_NoAllTarget(t *testing.T) {
	p := writeGoreleaser(t, `
version: 2
builds:
  - { id: evolve, targets: [ darwin_arm64, linux_amd64 ] }
archives:
  - { id: evolve, name_template: 'evolve_{{ .Os }}_{{ .Arch }}', formats: [ 'tar.gz' ] }
checksum: { name_template: 'checksums.txt' }
`)
	cfg, err := ParseConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, tg := range cfg.Targets {
		if tg.Arch == "all" {
			t.Errorf("no universal_binaries block, but a darwin_all target was invented: %+v", cfg.Targets)
		}
	}
}

func TestParseConfig_RepoFromReleaseGithub(t *testing.T) {
	p := writeGoreleaser(t, `
version: 2
builds:
  - { id: evolve, targets: [ darwin_arm64 ] }
archives:
  - { id: evolve, name_template: 'evolve_{{ .Os }}_{{ .Arch }}', formats: [ 'tar.gz' ] }
checksum: { name_template: 'checksums.txt' }
release:
  github:
    owner: mickeyyaya
    name: evolve-loop
`)
	cfg, err := ParseConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RepoOwner != "mickeyyaya" || cfg.RepoName != "evolve-loop" {
		t.Fatalf("repo = %q/%q, want mickeyyaya/evolve-loop", cfg.RepoOwner, cfg.RepoName)
	}
}

func TestTargetSplit_FirstUnderscore(t *testing.T) {
	p := writeGoreleaser(t, `
version: 2
builds:
  - id: evolve
    targets: [ linux_riscv64 ]
archives:
  - { id: evolve, name_template: 'evolve_{{ .Os }}_{{ .Arch }}', formats: [ 'tar.gz' ] }
checksum: { name_template: 'checksums.txt' }
`)
	cfg, err := ParseConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Targets) != 1 || cfg.Targets[0].OS != "linux" || cfg.Targets[0].Arch != "riscv64" {
		t.Fatalf("split wrong: %+v", cfg.Targets)
	}
}

func TestParseConfig_DefaultsChecksumsName(t *testing.T) {
	p := writeGoreleaser(t, `
version: 2
builds:
  - { id: evolve, targets: [ darwin_arm64 ] }
archives:
  - { id: evolve, name_template: 'evolve_{{ .Os }}_{{ .Arch }}', formats: [ 'tar.gz' ] }
`)
	cfg, err := ParseConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ChecksumsName != "checksums.txt" {
		t.Fatalf("ChecksumsName = %q, want defaulted checksums.txt", cfg.ChecksumsName)
	}
}

func TestParseConfig_Errors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		if _, err := ParseConfig(filepath.Join(t.TempDir(), "nope.yml")); err == nil {
			t.Fatal("want error for missing file")
		}
	})
	t.Run("no targets", func(t *testing.T) {
		p := writeGoreleaser(t, "version: 2\nbuilds:\n  - id: evolve\n")
		if _, err := ParseConfig(p); err == nil {
			t.Fatal("want error when zero targets declared")
		}
	})
	t.Run("malformed yaml", func(t *testing.T) {
		p := writeGoreleaser(t, "builds: [ : : : ]")
		if _, err := ParseConfig(p); err == nil {
			t.Fatal("want error for malformed yaml")
		}
	})
}

func TestParseConfig_RealRepo(t *testing.T) {
	root := repoRootForTest(t)
	cfg, err := ParseConfig(filepath.Join(root, ".goreleaser.yml"))
	if err != nil {
		t.Fatalf("parse real .goreleaser.yml: %v", err)
	}
	if len(cfg.Targets) < 4 {
		t.Fatalf("real config has %d targets, expected the full release matrix", len(cfg.Targets))
	}
	have := map[string]bool{}
	for _, tg := range cfg.Targets {
		name, err := cfg.AssetName(tg)
		if err != nil {
			t.Fatalf("AssetName(%v): %v", tg, err)
		}
		have[name] = true
	}
	for _, core := range []string{
		"evolve_darwin_amd64.tar.gz",
		"evolve_darwin_arm64.tar.gz",
		"evolve_linux_amd64.tar.gz",
		"evolve_linux_arm64.tar.gz",
		"evolve_darwin_all.tar.gz",
	} {
		if !have[core] {
			t.Errorf("core release asset %q missing from parsed matrix", core)
		}
	}
}

func repoRootForTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".goreleaser.yml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate .goreleaser.yml above %s", dir)
		}
		dir = parent
	}
}

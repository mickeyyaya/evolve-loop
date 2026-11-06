package sandbox

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestHomeStateWriteDirs_OmitStoredLoginDropsOnlyTheClaudeDirectory(t *testing.T) {
	t.Parallel()
	home := "/h"
	all, kept := homeStateWriteDirs(home, false), homeStateWriteDirs(home, true)
	claude := filepath.Join(home, ".claude")
	if !slices.Contains(all, claude) || slices.Contains(kept, claude) {
		t.Fatalf("all=%v kept=%v", all, kept)
	}
	if len(kept) != len(all)-1 {
		t.Fatalf("all=%v kept=%v", all, kept)
	}
}

func TestSandboxProfiles_OmitStoredLoginRemovesTheClaudeWriteGrantFromBothOSes(t *testing.T) {
	t.Parallel()
	for _, omit := range []bool{false, true} {
		cfg := Config{HomeDir: "/h", RepoRoot: "/r", OmitStoredLogin: omit}
		sbpl := GenerateSBPL(cfg)
		bwrap := strings.Join(BwrapPrefix(cfg), "\x00")
		hasSBPL := strings.Contains(sbpl, `(allow file-write* (subpath "/h/.claude"))`)
		hasBwrap := strings.Contains(bwrap, "--bind-try\x00/h/.claude\x00/h/.claude")
		if hasSBPL == omit || hasBwrap == omit {
			t.Errorf("omit=%v: sbpl grant=%v bwrap bind=%v", omit, hasSBPL, hasBwrap)
		}
	}
}

func TestSandboxProfiles_MaskFilesDenyReadAndReadOnlyFilesDenyWriteOnBothOSes(t *testing.T) {
	t.Parallel()
	cfg := Config{HomeDir: "/h", RepoRoot: "/r", MaskFiles: []string{"/h/.claude.json"}, ReadOnlyFiles: []string{"/h/.evolve-seat/settings.json"}}
	sbpl := GenerateSBPL(cfg)
	for _, want := range []string{
		`(deny file-read* (literal "/h/.claude.json"))`,
		`(deny file-write* (literal "/h/.evolve-seat/settings.json"))`,
	} {
		if !strings.Contains(sbpl, want) {
			t.Errorf("sbpl lacks %s", want)
		}
	}
	bwrap := strings.Join(BwrapPrefix(cfg), "\x00")
	for _, want := range []string{
		"--ro-bind\x00/dev/null\x00/h/.claude.json",
		"--ro-bind\x00/h/.evolve-seat/settings.json\x00/h/.evolve-seat/settings.json",
	} {
		if !strings.Contains(bwrap, want) {
			t.Errorf("bwrap lacks %q", want)
		}
	}
}

func TestSandboxProfiles_NoFileRowsMeansNoFileRules(t *testing.T) {
	t.Parallel()
	cfg := Config{HomeDir: "/h", RepoRoot: "/r"}
	if strings.Contains(GenerateSBPL(cfg), "(deny file-read* (literal") {
		t.Error("a config with no file rows must add no literal rule")
	}
	if strings.Contains(strings.Join(BwrapPrefix(cfg), "\x00"), "/dev/null") {
		t.Error("a config with no file rows must add no mask")
	}
}

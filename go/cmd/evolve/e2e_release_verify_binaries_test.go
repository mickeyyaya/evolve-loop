//go:build e2e

package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/releasetargets"
)

func TestReleaseVerifyBinaries_RealConfigAllPresent(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	cfg, err := releasetargets.ParseConfig(filepath.Join(repoRoot, ".goreleaser.yml"))
	if err != nil {
		t.Fatalf("parse real .goreleaser.yml: %v", err)
	}

	// A lister that returns exactly the assets the SSOT implies — the "happy"
	// published release — must produce an all-OK matrix.
	list := func(owner, repo, tag string) ([]string, error) {
		names := []string{cfg.ChecksumsName}
		for _, tg := range cfg.Targets {
			n, err := cfg.AssetName(tg)
			if err != nil {
				t.Fatalf("AssetName(%v): %v", tg, err)
			}
			names = append(names, n)
		}
		return names, nil
	}

	rows, err := verifyReleaseBinaries(cfg, "v0.0.0-test", list)
	if err != nil {
		t.Fatalf("verifyReleaseBinaries: %v", err)
	}
	if len(rows) != len(cfg.Targets)+1 {
		t.Fatalf("want %d rows (targets + checksums), got %d", len(cfg.Targets)+1, len(rows))
	}
	for _, r := range rows {
		if !r.OK {
			t.Errorf("%s: not OK against the SSOT-derived release: %s", r.Asset, r.Detail)
		}
	}
}

// TestReleaseVerifyBinaries_BinaryDispatch runs with no tag arg so it stays
// network-free.
func TestReleaseVerifyBinaries_BinaryDispatch(t *testing.T) {
	if testing.Short() {
		t.Skip("E2E test (builds the evolve binary); skipped in -short mode")
	}
	repoRoot := mustRepoRoot(t)
	binPath := buildBinary(t, t.TempDir(), "evolve", "./cmd/evolve", repoRoot)

	out, err := exec.Command(binPath, "release-verify-binaries").CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit with no tag arg; output: %s", out)
	}
	got := string(out)
	if strings.Contains(got, "unknown command") {
		t.Fatalf("dispatcher did not recognize release-verify-binaries: %s", got)
	}
	if !strings.Contains(got, "usage:") || !strings.Contains(got, "tag") {
		t.Fatalf("expected usage/tag guidance, got: %s", got)
	}
}

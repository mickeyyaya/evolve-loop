//go:build e2e

// End-to-end proof that release-verify-binaries integrates with the REAL
// .goreleaser.yml SSOT and that the subcommand is wired into the dispatcher.
//
// Why this tier: the unit tests in cmd_release_verify_binaries_test.go drive the
// orchestration with a fixture Config, so they say nothing about whether the
// real goreleaser config parses or whether the binary actually recognizes the
// subcommand. These tests close both gaps without touching the network.
package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/releasetargets"
)

// TestReleaseVerifyBinaries_RealConfigAllPresent parses the checked-in
// .goreleaser.yml (the SSOT) and verifies the orchestration reports every target
// OK when the release publishes exactly the assets the config implies. This is
// the deterministic form of "all prebuilt binaries are accounted for".
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

// TestReleaseVerifyBinaries_BinaryDispatch builds the real binary and proves the
// subcommand is recognized by the dispatcher (NOT "unknown command") via the
// usage path — exercised with no tag so it stays network-free.
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

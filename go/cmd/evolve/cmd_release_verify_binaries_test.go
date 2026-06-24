package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/releasetargets"
)

// fixtureConfig is a small but realistic release matrix used to drive the
// verifier orchestration without touching the network or the real config.
func fixtureConfig() releasetargets.Config {
	return releasetargets.Config{
		Targets: []releasetargets.Target{
			{OS: "darwin", Arch: "amd64"},
			{OS: "linux", Arch: "arm64"},
		},
		ArchiveNameTemplate: "evolve_{{ .Os }}_{{ .Arch }}",
		ArchiveFormat:       "tar.gz",
		ChecksumsName:       "checksums.txt",
		RepoOwner:           "mickeyyaya",
		RepoName:            "evolve-loop",
	}
}

func rowsByAsset(rows []binVerify) map[string]binVerify {
	m := make(map[string]binVerify, len(rows))
	for _, r := range rows {
		m[r.Asset] = r
	}
	return m
}

func TestVerifyReleaseBinaries_AllPresent(t *testing.T) {
	cfg := fixtureConfig()
	list := func(owner, repo, tag string) ([]string, error) {
		if owner != "mickeyyaya" || repo != "evolve-loop" || tag != "v9.9.9" {
			return nil, fmt.Errorf("unexpected call %s/%s@%s", owner, repo, tag)
		}
		return []string{
			"evolve_darwin_amd64.tar.gz",
			"evolve_linux_arm64.tar.gz",
			"checksums.txt",
		}, nil
	}
	rows, err := verifyReleaseBinaries(cfg, "v9.9.9", list)
	if err != nil {
		t.Fatalf("verifyReleaseBinaries: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows (2 targets + checksums), got %d: %+v", len(rows), rows)
	}
	for _, r := range rows {
		if !r.OK {
			t.Errorf("row %q expected OK, got FAIL: %s", r.Asset, r.Detail)
		}
	}
}

func TestVerifyReleaseBinaries_MissingArchive(t *testing.T) {
	cfg := fixtureConfig()
	// linux_arm64 archive absent — its row must FAIL, the others stay OK
	// (no early return: the operator sees the full picture).
	list := func(owner, repo, tag string) ([]string, error) {
		return []string{"evolve_darwin_amd64.tar.gz", "checksums.txt"}, nil
	}
	rows, err := verifyReleaseBinaries(cfg, "v1.0.0", list)
	if err != nil {
		t.Fatalf("verifyReleaseBinaries: %v", err)
	}
	by := rowsByAsset(rows)
	if r := by["evolve_linux_arm64.tar.gz"]; r.OK {
		t.Errorf("missing archive should FAIL, got OK")
	}
	if r := by["evolve_darwin_amd64.tar.gz"]; !r.OK {
		t.Errorf("present archive should stay OK")
	}
	if r := by["checksums.txt"]; !r.OK {
		t.Errorf("checksums present should stay OK")
	}
}

func TestVerifyReleaseBinaries_MissingChecksums(t *testing.T) {
	cfg := fixtureConfig()
	list := func(owner, repo, tag string) ([]string, error) {
		return []string{"evolve_darwin_amd64.tar.gz", "evolve_linux_arm64.tar.gz"}, nil
	}
	rows, err := verifyReleaseBinaries(cfg, "v1.0.0", list)
	if err != nil {
		t.Fatal(err)
	}
	if r := rowsByAsset(rows)["checksums.txt"]; r.OK {
		t.Errorf("missing checksums.txt should FAIL, got OK")
	}
}

func TestVerifyReleaseBinaries_ListerError(t *testing.T) {
	cfg := fixtureConfig()
	list := func(owner, repo, tag string) ([]string, error) {
		return nil, fmt.Errorf("release v0.0.0 not found")
	}
	if _, err := verifyReleaseBinaries(cfg, "v0.0.0", list); err == nil {
		t.Fatal("want error when the release cannot be listed (no assets published)")
	}
}

func TestReportBinaryVerification_AllOK(t *testing.T) {
	cfg := fixtureConfig()
	list := func(owner, repo, tag string) ([]string, error) {
		return []string{
			"evolve_darwin_amd64.tar.gz",
			"evolve_linux_arm64.tar.gz",
			"checksums.txt",
		}, nil
	}
	var out, errb strings.Builder
	rc := reportBinaryVerification(cfg, "v1.2.3", list, &out, &errb)
	if rc != 0 {
		t.Fatalf("want exit 0 when all present, got %d (stderr=%q)", rc, errb.String())
	}
	if !strings.Contains(out.String(), "OK") {
		t.Errorf("table should report OK rows, got %q", out.String())
	}
}

func TestReportBinaryVerification_SomeMissing(t *testing.T) {
	cfg := fixtureConfig()
	list := func(owner, repo, tag string) ([]string, error) {
		return []string{"evolve_darwin_amd64.tar.gz"}, nil // arm64 + checksums missing
	}
	var out, errb strings.Builder
	rc := reportBinaryVerification(cfg, "v1.2.3", list, &out, &errb)
	if rc == 0 {
		t.Fatalf("want non-zero exit when assets missing")
	}
	if !strings.Contains(out.String(), "FAIL") {
		t.Errorf("table should mark missing assets FAIL, got %q", out.String())
	}
}

func TestReportBinaryVerification_ListerError(t *testing.T) {
	cfg := fixtureConfig()
	list := func(owner, repo, tag string) ([]string, error) {
		return nil, fmt.Errorf("release not found")
	}
	var out, errb strings.Builder
	if rc := reportBinaryVerification(cfg, "v0.0.0", list, &out, &errb); rc == 0 {
		t.Fatalf("want non-zero exit when the release cannot be listed")
	}
}

func TestRunReleaseVerifyBinaries_MissingTagArg(t *testing.T) {
	var out, errb strings.Builder
	rc := runReleaseVerifyBinaries(nil, nil, &out, &errb)
	if rc == 0 {
		t.Fatalf("want non-zero exit when tag arg missing, got 0")
	}
	if !strings.Contains(errb.String(), "tag") {
		t.Errorf("stderr should explain the missing tag arg, got %q", errb.String())
	}
}

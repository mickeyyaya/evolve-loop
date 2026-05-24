package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeMarketplace mirrors the package test helper so the cmd tests stay
// independent of internal/marketplacepoll exports.
func cmdMakeMarketplace(t *testing.T, version string) string {
	t.Helper()
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := fmt.Sprintf(`{"name":"evolve-loop","version":"%s"}`, version)
	if err := os.WriteFile(filepath.Join(d, ".claude-plugin", "plugin.json"),
		[]byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return d
}

func TestCmd_MarketplacePoll_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runMarketplacePoll([]string{"--help"}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if !strings.Contains(stdout.String(), "marketplace-poll") {
		t.Errorf("help missing keyword: %s", stdout.String())
	}
}

func TestCmd_MarketplacePoll_NoTarget(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runMarketplacePoll(nil, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10", rc)
	}
}

func TestCmd_MarketplacePoll_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runMarketplacePoll([]string{"--bogus"}, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10", rc)
	}
}

func TestCmd_MarketplacePoll_BadMaxWait(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runMarketplacePoll([]string{"--max-wait-s", "abc", "1.0.0"}, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10", rc)
	}
}

func TestCmd_MarketplacePoll_BadPollInterval(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runMarketplacePoll([]string{"--poll-interval-s", "0", "1.0.0"}, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10", rc)
	}
}

func TestCmd_MarketplacePoll_DryRun(t *testing.T) {
	m := cmdMakeMarketplace(t, "0.9.0")
	var stdout, stderr bytes.Buffer
	rc := runMarketplacePoll([]string{
		"1.2.3",
		"--marketplace-dir", m,
		"--max-wait-s", "2",
		"--poll-interval-s", "1",
		"--dry-run",
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0\nstderr=%s", rc, stderr.String())
	}
	if !strings.Contains(stderr.String(), "DRY-RUN") {
		t.Errorf("stderr missing DRY-RUN: %s", stderr.String())
	}
}

func TestCmd_MarketplacePoll_MissingDir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runMarketplacePoll([]string{
		"1.0.0",
		"--marketplace-dir", "/tmp/does-not-exist-mkpoll-cmd-xyz",
		"--max-wait-s", "1",
		"--poll-interval-s", "1",
	}, nil, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("rc = %d, want 2\nstderr=%s", rc, stderr.String())
	}
}

func TestCmd_MarketplacePoll_ExtraPositional(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runMarketplacePoll([]string{"1.0.0", "extra"}, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10", rc)
	}
}

// TestCmd_MarketplacePoll_BadSemver verifies that a non-semver target
// surfaces as exit 2 (matches bash test 9: "non-semver target → exit 2").
func TestCmd_MarketplacePoll_BadSemver(t *testing.T) {
	m := cmdMakeMarketplace(t, "1.0.0")
	var stdout, stderr bytes.Buffer
	rc := runMarketplacePoll([]string{
		"garbage",
		"--marketplace-dir", m,
		"--max-wait-s", "2",
		"--poll-interval-s", "1",
	}, nil, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("rc = %d, want 2\nstderr=%s", rc, stderr.String())
	}
}

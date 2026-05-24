package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCmd_ReleasePreflight_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runReleasePreflight([]string{"--help"}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if !strings.Contains(stdout.String(), "release-preflight") {
		t.Errorf("help missing keyword: %s", stdout.String())
	}
}

func TestCmd_ReleasePreflight_NoTarget(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runReleasePreflight(nil, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10", rc)
	}
}

func TestCmd_ReleasePreflight_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runReleasePreflight([]string{"--bogus", "1.0.0"}, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10", rc)
	}
}

func TestCmd_ReleasePreflight_ExtraPositional(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runReleasePreflight([]string{"1.0.0", "extra"}, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10", rc)
	}
}

func TestCmd_ReleasePreflight_MissingProjectRoot(t *testing.T) {
	// With no repo state, step 1 will fail (no git). The cmd should
	// surface exit 1, not 10.
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	var stdout, stderr bytes.Buffer
	rc := runReleasePreflight([]string{"1.0.0", "--dry-run", "--skip-tests"}, nil, &stdout, &stderr)
	// Dry-run bypasses git but step 3 will fail because no plugin.json.
	if rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
}

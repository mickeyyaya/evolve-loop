package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCmd_PruneEphemeral_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runPruneEphemeral([]string{"--help"}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if !strings.Contains(stdout.String(), "prune-ephemeral") {
		t.Errorf("help missing keyword: %s", stdout.String())
	}
}

func TestCmd_PruneEphemeral_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runPruneEphemeral([]string{"--bogus"}, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10", rc)
	}
}

func TestCmd_PruneEphemeral_UnexpectedArg(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runPruneEphemeral([]string{"extra"}, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10", rc)
	}
}

func TestCmd_PruneEphemeral_BadTrackerTTLFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runPruneEphemeral([]string{"--tracker-ttl-days=abc"}, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10 (bad --tracker-ttl-days flag)", rc)
	}
}

func TestCmd_PruneEphemeral_BadLogTTLFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runPruneEphemeral([]string{"--dispatch-log-ttl-days=xyz"}, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10 (bad --dispatch-log-ttl-days flag)", rc)
	}
}

func TestCmd_PruneEphemeral_HappyPath(t *testing.T) {
	d := t.TempDir()
	// Set up stale ephemeral so something is prunable.
	cyclePath := filepath.Join(d, ".evolve", "runs", "cycle-1", ".ephemeral")
	if err := os.MkdirAll(cyclePath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stale := time.Now().Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(cyclePath, stale, stale); err != nil {
		t.Fatalf("chtime: %v", err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", d)

	var stdout, stderr bytes.Buffer
	rc := runPruneEphemeral([]string{"--dry-run"}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0\nstderr=%s", rc, stderr.String())
	}
	if !strings.Contains(stderr.String(), "DRY-RUN would remove") {
		t.Errorf("stderr missing DRY-RUN log: %s", stderr.String())
	}
	// Dry-run must NOT have removed the dir.
	if _, err := os.Stat(cyclePath); err != nil {
		t.Errorf("dry-run removed the dir: %v", err)
	}
}

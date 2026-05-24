package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const cmdJournalFull = `{
  "version": "1.2.3",
  "tag": "v1.2.3",
  "commit_sha": "abcdef1234567890",
  "branch": "main"
}`

func TestCmd_Rollback_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runRollback([]string{"--help"}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if !strings.Contains(stdout.String(), "rollback") {
		t.Errorf("help missing keyword: %s", stdout.String())
	}
}

func TestCmd_Rollback_NoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runRollback(nil, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10", rc)
	}
}

func TestCmd_Rollback_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runRollback([]string{"--bogus"}, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10", rc)
	}
}

func TestCmd_Rollback_ReasonMissingValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runRollback([]string{"--reason"}, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10", rc)
	}
}

func TestCmd_Rollback_ExtraPositional(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runRollback([]string{"j1.json", "j2.json"}, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10", rc)
	}
}

func TestCmd_Rollback_MissingJournal(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runRollback([]string{"/tmp/does-not-exist-rollback-cmd-xyz.json"}, nil, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("rc = %d, want 2\nstderr=%s", rc, stderr.String())
	}
}

func TestCmd_Rollback_DryRun(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "journal.json")
	if err := os.WriteFile(p, []byte(cmdJournalFull), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	var stdout, stderr bytes.Buffer
	rc := runRollback([]string{p, "--dry-run", "--reason", "test"}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0\nstderr=%s", rc, stderr.String())
	}
	if !strings.Contains(stderr.String(), "DRY-RUN") {
		t.Errorf("stderr missing DRY-RUN: %s", stderr.String())
	}
}

func TestCmd_Rollback_MalformedJournal(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "bad.json")
	if err := os.WriteFile(p, []byte(`{"version":"1.0.0"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var stdout, stderr bytes.Buffer
	rc := runRollback([]string{p}, nil, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("rc = %d, want 2 for malformed journal", rc)
	}
}

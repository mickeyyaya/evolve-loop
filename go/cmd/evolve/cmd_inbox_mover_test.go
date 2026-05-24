package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupInbox(t *testing.T, taskID string) string {
	t.Helper()
	d := t.TempDir()
	inbox := filepath.Join(d, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := fmt.Sprintf(`{"id":"%s","payload":"x"}`, taskID)
	if err := os.WriteFile(filepath.Join(inbox, taskID+".json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return d
}

func TestCmd_InboxMover_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runInboxMover([]string{"--help"}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if !strings.Contains(stdout.String(), "claim") || !strings.Contains(stdout.String(), "promote") {
		t.Errorf("help missing subcommands: %s", stdout.String())
	}
}

func TestCmd_InboxMover_NoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runInboxMover(nil, nil, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
}

func TestCmd_InboxMover_UnknownSubcmd(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runInboxMover([]string{"bogus"}, nil, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
}

func TestCmd_InboxMover_Claim_HappyPath(t *testing.T) {
	d := setupInbox(t, "task-1")
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	var stdout, stderr bytes.Buffer
	rc := runInboxMover([]string{"claim", "task-1", "5"}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0\nstderr=%s", rc, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(d, ".evolve", "inbox", "processing", "cycle-5", "task-1.json")); err != nil {
		t.Errorf("dest file missing: %v", err)
	}
}

func TestCmd_InboxMover_Claim_MissingArgs(t *testing.T) {
	d := setupInbox(t, "task-1")
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	var stdout, stderr bytes.Buffer
	rc := runInboxMover([]string{"claim", "task-1"}, nil, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
}

func TestCmd_InboxMover_Claim_NotFound(t *testing.T) {
	d := setupInbox(t, "task-1")
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	var stdout, stderr bytes.Buffer
	rc := runInboxMover([]string{"claim", "task-missing", "5"}, nil, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
}

func TestCmd_InboxMover_Promote_HappyPath(t *testing.T) {
	d := t.TempDir()
	procDir := filepath.Join(d, ".evolve", "inbox", "processing", "cycle-5")
	if err := os.MkdirAll(procDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(procDir, "task-1.json"), []byte(`{"id":"task-1"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", d)

	var stdout, stderr bytes.Buffer
	rc := runInboxMover([]string{"promote", "task-1", "processed", "5"}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0\nstderr=%s", rc, stderr.String())
	}
}

func TestCmd_InboxMover_Promote_NoOpExitsZero(t *testing.T) {
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	var stdout, stderr bytes.Buffer
	rc := runInboxMover([]string{"promote", "task-missing", "processed"}, nil, &stdout, &stderr)
	// ship.sh compat: missing task is NoOp exit 0, not error.
	if rc != 0 {
		t.Fatalf("rc = %d, want 0 (NoOp)", rc)
	}
}

func TestCmd_InboxMover_Promote_BadState(t *testing.T) {
	d := setupInbox(t, "task-1")
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	var stdout, stderr bytes.Buffer
	rc := runInboxMover([]string{"promote", "task-1", "bogus-state"}, nil, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("rc = %d, want 1 for bad state", rc)
	}
}

func TestCmd_InboxMover_Promote_CommitSHAFlag(t *testing.T) {
	d := t.TempDir()
	procDir := filepath.Join(d, ".evolve", "inbox", "processing", "cycle-5")
	if err := os.MkdirAll(procDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(procDir, "task-1.json"), []byte(`{"id":"task-1"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	var stdout, stderr bytes.Buffer
	rc := runInboxMover([]string{"promote", "task-1", "processed", "5", "--commit-sha", "deadbeef1234"}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	// File should have SHA8 prefix.
	dest := filepath.Join(d, ".evolve", "inbox", "processed", "cycle-5", "deadbeef-task-1.json")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("dest with SHA prefix missing: %v", err)
	}
}

func TestCmd_InboxMover_RecoverOrphans_NoOp(t *testing.T) {
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	var stdout, stderr bytes.Buffer
	rc := runInboxMover([]string{"recover-orphans"}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
}

func TestParsePromoteArgs(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		wantCycle string
		wantSHA   string
	}{
		{"empty", nil, "", ""},
		{"cycle only", []string{"7"}, "7", ""},
		{"sha only", []string{"--commit-sha", "abc"}, "", "abc"},
		{"both", []string{"7", "--commit-sha", "abc"}, "7", "abc"},
		{"sha then cycle", []string{"--commit-sha", "abc", "7"}, "7", "abc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := parsePromoteArgs(tc.args)
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if p.Cycle != tc.wantCycle || p.CommitSHA != tc.wantSHA {
				t.Errorf("got %+v, want cycle=%q sha=%q", p, tc.wantCycle, tc.wantSHA)
			}
		})
	}
}

func TestParsePromoteArgs_SHAMissingValue(t *testing.T) {
	_, err := parsePromoteArgs([]string{"--commit-sha"})
	if err == nil {
		t.Error("want err for --commit-sha without value")
	}
}

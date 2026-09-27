package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// findRepoRootForBinding walks up from CWD to the directory holding agents/
// (the commitprefixgate manifest_realpaths_test.go idiom).
func findRepoRootForBinding(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "agents", "evolve-triage.md")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Skip("repo root with agents/evolve-triage.md not found (bare test env)")
	return ""
}

func TestTriageAgentDoc_ClaimBindsGoInboxMover(t *testing.T) {
	root := findRepoRootForBinding(t)
	doc, err := os.ReadFile(filepath.Join(root, "agents", "evolve-triage.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(doc)
	if strings.Contains(s, "inbox-mover.sh") {
		t.Error("agents/evolve-triage.md still invokes the deleted inbox-mover.sh — claim never reaches the Go floor (ErrConsoleRouted unreachable)")
	}
	if !strings.Contains(s, "evolve inbox-mover claim") {
		t.Error("agents/evolve-triage.md must instruct `evolve inbox-mover claim` so claims route through inboxmover.Claim")
	}
}

func TestTriageProfile_AllowlistsGoInboxMover(t *testing.T) {
	root := findRepoRootForBinding(t)
	prof, err := os.ReadFile(filepath.Join(root, ".evolve", "profiles", "triage.json"))
	if err != nil {
		t.Skip("triage profile absent in this checkout")
	}
	s := string(prof)
	if strings.Contains(s, "inbox-mover.sh") {
		t.Error(".evolve/profiles/triage.json allowlists the deleted inbox-mover.sh — permission layer blocks the Go claim command")
	}
	if !strings.Contains(s, "evolve inbox-mover") {
		t.Error(".evolve/profiles/triage.json must allowlist `evolve inbox-mover` for the claim floor to be invocable")
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func plantSession(t *testing.T, root, session string) string {
	t.Helper()
	dir := filepath.Join(root, "projects", "-wt-cycle-9")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, session+".jsonl")
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFindSessionTranscript_FindsASessionInTheStoredRootOrTheSeatRoot(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	home := t.TempDir()
	stored := plantSession(t, filepath.Join(home, ".claude"), "stored-session")
	seat := plantSession(t, filepath.Join(home, ".evolve-seat"), "seat-session")
	for session, want := range map[string]string{"stored-session": stored, "seat-session": seat} {
		got, err := findSessionTranscript(home, session)
		if err != nil || got != want {
			t.Fatalf("findSessionTranscript(%q) = %q, %v; want %q", session, got, err, want)
		}
	}
}

func TestFindSessionTranscript_NamesEveryRootWhenTheSessionIsAbsent(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	home := t.TempDir()
	plantSession(t, filepath.Join(home, ".claude"), "other")
	_, err := findSessionTranscript(home, "missing")
	if err == nil {
		t.Fatal("an absent session must be an error")
	}
	for _, root := range []string{filepath.Join(home, ".claude"), filepath.Join(home, ".evolve-seat")} {
		if !strings.Contains(err.Error(), root) {
			t.Fatalf("the error must name the root %s it searched: %v", root, err)
		}
	}
}

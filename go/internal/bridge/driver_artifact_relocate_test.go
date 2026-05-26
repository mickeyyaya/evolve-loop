package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

// driver_artifact_relocate_test.go — artifactReady tolerance for the
// cycle-108 ExitArtifactTimeout root cause: agents intermittently wrote the
// report to <workspace>/workspace/<file> (reading the doc's "workspace/"
// prefix as a literal subdir) while the driver polls only the canonical
// <workspace>/<file>. artifactReady accepts either location and relocates the
// non-canonical write so downstream phases — which read the canonical path —
// still resolve it. See docs/architecture/adr/0024-*.md (Step 0).

func TestArtifactReady_CanonicalPresent(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "scout-report.md")
	if err := os.WriteFile(canonical, []byte("## Proposed Tasks\n"), 0o644); err != nil {
		t.Fatalf("seed canonical: %v", err)
	}
	cfg := &Config{Workspace: ws, Artifact: canonical}

	ready, from := artifactReady(cfg)
	if !ready {
		t.Fatal("artifactReady should be true when the canonical artifact is present")
	}
	if from != "" {
		t.Fatalf("no relocation expected when canonical present; got from=%q", from)
	}
}

func TestArtifactReady_RelocatesFromWorkspaceSubdir(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "scout-report.md")
	subdir := filepath.Join(ws, "workspace", "scout-report.md")
	if err := os.MkdirAll(filepath.Dir(subdir), 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	body := []byte("## Proposed Tasks\n1. do the thing\n")
	if err := os.WriteFile(subdir, body, 0o644); err != nil {
		t.Fatalf("seed subdir: %v", err)
	}
	cfg := &Config{Workspace: ws, Artifact: canonical}

	ready, from := artifactReady(cfg)
	if !ready {
		t.Fatal("artifactReady should be true when the artifact is in the workspace/ subdir")
	}
	if from != subdir {
		t.Fatalf("relocatedFrom = %q, want %q", from, subdir)
	}
	// The content must now live at the canonical path the driver/runner read.
	got, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatalf("canonical not present after relocation: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("relocated content = %q, want %q", got, body)
	}
	// The non-canonical copy must be gone (single source of truth).
	if _, err := os.Stat(subdir); !os.IsNotExist(err) {
		t.Fatalf("subdir copy should be removed after relocation; stat err=%v", err)
	}
}

func TestArtifactReady_NeitherPresent(t *testing.T) {
	ws := t.TempDir()
	cfg := &Config{Workspace: ws, Artifact: filepath.Join(ws, "scout-report.md")}

	ready, from := artifactReady(cfg)
	if ready || from != "" {
		t.Fatalf("artifactReady should be (false, \"\") when no artifact exists; got (%v, %q)", ready, from)
	}
}

func TestArtifactReady_EmptyFilesDoNotCount(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "scout-report.md")
	subdir := filepath.Join(ws, "workspace", "scout-report.md")
	if err := os.WriteFile(canonical, nil, 0o644); err != nil {
		t.Fatalf("seed empty canonical: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(subdir), 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	if err := os.WriteFile(subdir, nil, 0o644); err != nil {
		t.Fatalf("seed empty subdir: %v", err)
	}
	cfg := &Config{Workspace: ws, Artifact: canonical}

	ready, from := artifactReady(cfg)
	if ready || from != "" {
		t.Fatalf("empty artifacts must not count as ready; got (%v, %q)", ready, from)
	}
}

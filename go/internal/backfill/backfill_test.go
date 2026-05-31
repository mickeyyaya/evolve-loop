package backfill

// RED tests for cycle-171 T3 (artifact-backfill). The backfill package does not
// exist yet, so this test file fails to BUILD at the baseline (undefined:
// TryExtract) — the correct RED for a brand-new package, isolated to this
// package's own test binary. Builder makes it GREEN by creating backfill.go with
//
//	func TryExtract(workspace, phase, artifactPath string, minLen int) (bool, error)
//
// which reads <workspace>/<phase>-stdout.clean.txt, finds the LAST occurrence of
// the phase's markdown header (scout→"# Scout Report", build→"# Build Report",
// audit→"# Audit Report", tdd→"# TDD", intent→"# Intent", triage→"# Triage"),
// extracts header..EOF (trimmed), and — only when len >= minLen — writes it to
// artifactPath atomically. Returns (true,nil) on success, (false,nil) on
// no-header / too-short / unknown-phase, (false,err) on I/O error.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedClean(t *testing.T, ws, phase, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(ws, phase+"-stdout.clean.txt"), []byte(body), 0o644); err != nil {
		t.Fatalf("seed %s-stdout.clean.txt: %v", phase, err)
	}
}

// AC-2 positive: header present + content above minLen → extracted=true and the
// artifact file is written containing the reconstructed report.
func TestTryExtract_PositiveWritesArtifact(t *testing.T) {
	ws := t.TempDir()
	report := "# Scout Report\n\nReconstructed scout body long enough to clear the minimum-length floor for backfill extraction.\n"
	// Header must be found even with leading log noise; "last occurrence" wins.
	seedClean(t, ws, "scout", "spinner noise line\nmore stdout chatter\n"+report)
	artifact := filepath.Join(ws, "scout-report.md")

	ok, err := TryExtract(ws, "scout", artifact, 50)
	if err != nil {
		t.Fatalf("TryExtract returned err: %v", err)
	}
	if !ok {
		t.Fatalf("TryExtract should report extracted=true for a present header with long content")
	}
	got, rerr := os.ReadFile(artifact)
	if rerr != nil {
		t.Fatalf("artifact must be written on successful extract: %v", rerr)
	}
	if !strings.Contains(string(got), "# Scout Report") {
		t.Errorf("extracted artifact must contain the phase header; got %q", got)
	}
}

// AC-3 dimension: header absent → extracted=false, no artifact written.
func TestTryExtract_NoHeaderReturnsFalse(t *testing.T) {
	ws := t.TempDir()
	seedClean(t, ws, "scout", "just spinner noise, no markdown header at all, plenty of bytes here\n")
	artifact := filepath.Join(ws, "scout-report.md")

	ok, err := TryExtract(ws, "scout", artifact, 50)
	if err != nil {
		t.Fatalf("TryExtract returned err: %v", err)
	}
	if ok {
		t.Errorf("no header present → extracted must be false")
	}
	if _, serr := os.Stat(artifact); !os.IsNotExist(serr) {
		t.Errorf("artifact must NOT be written when no header is found")
	}
}

// AC-4 dimension: header present but body below minLen → extracted=false, no write.
func TestTryExtract_TooShortReturnsFalse(t *testing.T) {
	ws := t.TempDir()
	seedClean(t, ws, "scout", "# Scout Report\nshort")
	artifact := filepath.Join(ws, "scout-report.md")

	ok, err := TryExtract(ws, "scout", artifact, 200)
	if err != nil {
		t.Fatalf("TryExtract returned err: %v", err)
	}
	if ok {
		t.Errorf("content below minLen → extracted must be false")
	}
	if _, serr := os.Stat(artifact); !os.IsNotExist(serr) {
		t.Errorf("artifact must NOT be written when content is below minLen")
	}
}

// AC-3 dimension: a phase with no header mapping → extracted=false, no error.
func TestTryExtract_UnknownPhaseReturnsFalse(t *testing.T) {
	ws := t.TempDir()
	seedClean(t, ws, "bogus", "# Bogus Header\n"+strings.Repeat("x", 300))
	artifact := filepath.Join(ws, "bogus.md")

	ok, err := TryExtract(ws, "bogus", artifact, 10)
	if err != nil {
		t.Fatalf("unknown phase must not error, got %v", err)
	}
	if ok {
		t.Errorf("unknown phase (no header in the map) → extracted must be false")
	}
}

// Edge axis: clean.txt absent → extracted=false (no panic). Whether a missing
// source file is reported as an error is left to the implementation; this test
// only pins the observable extracted=false.
func TestTryExtract_MissingCleanFileReturnsFalse(t *testing.T) {
	ws := t.TempDir() // no clean.txt seeded
	artifact := filepath.Join(ws, "scout-report.md")

	ok, _ := TryExtract(ws, "scout", artifact, 50)
	if ok {
		t.Errorf("missing clean.txt → extracted must be false")
	}
	if _, serr := os.Stat(artifact); !os.IsNotExist(serr) {
		t.Errorf("artifact must NOT be written when clean.txt is missing")
	}
}

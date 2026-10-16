package backfill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTryExtract_WriteErrorReturnsWrappedError(t *testing.T) {
	ws := t.TempDir()
	report := "# Scout Report\n\nReconstructed scout body long enough to clear the minimum-length floor.\n"
	seedClean(t, ws, "scout", report)
	blocker := filepath.Join(ws, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed blocker file: %v", err)
	}
	artifact := filepath.Join(blocker, "scout-report.md")

	ok, err := TryExtract(ws, "scout", artifact, 50)
	if ok {
		t.Errorf("TryExtract must report extracted=false when the artifact write fails")
	}
	if err == nil {
		t.Fatalf("TryExtract must return an error when the artifact write fails")
	}
	if !strings.Contains(err.Error(), "backfill write") {
		t.Errorf("error must carry the backfill-write context; got %q", err.Error())
	}
	if !strings.Contains(err.Error(), artifact) {
		t.Errorf("error must name the target artifact path %q; got %q", artifact, err.Error())
	}
	if _, serr := os.Stat(artifact); serr == nil {
		t.Errorf("no artifact must be left behind when the write fails")
	}
}

func seedClean(t *testing.T, ws, phase, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(ws, phase+"-stdout.clean.txt"), []byte(body), 0o644); err != nil {
		t.Fatalf("seed %s-stdout.clean.txt: %v", phase, err)
	}
}

func TestTryExtract_PositiveWritesArtifact(t *testing.T) {
	ws := t.TempDir()
	report := "# Scout Report\n\nReconstructed scout body long enough to clear the minimum-length floor for backfill extraction.\n"
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
	if _, serr := os.Stat(artifact); serr == nil {
		t.Errorf("artifact must NOT be written when no header is found")
	}
}

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
	if _, serr := os.Stat(artifact); serr == nil {
		t.Errorf("artifact must NOT be written when content is below minLen")
	}
}

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

func TestTryExtract_MissingCleanFileReturnsFalse(t *testing.T) {
	ws := t.TempDir()
	artifact := filepath.Join(ws, "scout-report.md")

	ok, _ := TryExtract(ws, "scout", artifact, 50)
	if ok {
		t.Errorf("missing clean.txt → extracted must be false")
	}
	if _, serr := os.Stat(artifact); serr == nil {
		t.Errorf("artifact must NOT be written when clean.txt is missing")
	}
}

func TestTryExtract_Retro_PositiveWritesArtifact(t *testing.T) {
	ws := t.TempDir()
	report := "# Retrospective Report — Cycle 187\n\n## Verdict trigger\n" +
		"Auditor verdict: FAIL. Root cause reconstructed from stdout for backfill recovery.\n"
	seedClean(t, ws, "retro", "bridge boot timeout noise\nspinner chatter line\n"+report)
	artifact := filepath.Join(ws, "retrospective-report.md")

	ok, err := TryExtract(ws, "retro", artifact, 50)
	if err != nil {
		t.Fatalf("TryExtract(retro) returned err: %v", err)
	}
	if !ok {
		t.Fatalf("retro must be a backfill-covered phase: expected extracted=true for a present # Retrospective Report header")
	}
	got, rerr := os.ReadFile(artifact)
	if rerr != nil {
		t.Fatalf("retro artifact must be written on successful extract: %v", rerr)
	}
	if !strings.Contains(string(got), "# Retrospective Report") {
		t.Errorf("extracted retro artifact must contain its header; got %q", got)
	}
}

func TestTryExtract_BuildPlanner_PositiveWritesArtifact(t *testing.T) {
	ws := t.TempDir()
	report := "# Build Plan — Cycle 187\n\n## Approach\n" +
		"File-by-file implementation plan reconstructed from build-planner stdout, long enough to clear minLen.\n"
	seedClean(t, ws, "build-planner", "planner spinner line\n"+report)
	artifact := filepath.Join(ws, "build-plan.md")

	ok, err := TryExtract(ws, "build-planner", artifact, 50)
	if err != nil {
		t.Fatalf("TryExtract(build-planner) returned err: %v", err)
	}
	if !ok {
		t.Fatalf("build-planner must be a backfill-covered phase: expected extracted=true for a present # Build Plan header")
	}
	got, rerr := os.ReadFile(artifact)
	if rerr != nil {
		t.Fatalf("build-plan.md must be written on successful extract: %v", rerr)
	}
	if !strings.Contains(string(got), "# Build Plan") {
		t.Errorf("extracted build-plan artifact must contain its header; got %q", got)
	}
}

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

// --- cycle-187: backfill coverage extended to retro + build-planner ---

// AC-1 / AC-7 (cycle-187): "retro" is a newly backfill-covered phase. The
// retrospective agent's stdout carries the canonical "# Retrospective Report"
// header — the exact form every real retrospective-report.md on disk uses
// (e.g. ".evolve/runs/cycle-29/retrospective-report.md" → "# Retrospective
// Report — Cycle 29"). With "retro" in phaseHeaders, that report must be
// reconstructed and written to the artifact path.
//
// RED at baseline: phaseHeaders has no "retro" entry → TryExtract takes the
// unknown-phase branch and returns (false, nil) → ok==false, artifact absent.
func TestTryExtract_Retro_PositiveWritesArtifact(t *testing.T) {
	ws := t.TempDir()
	report := "# Retrospective Report — Cycle 187\n\n## Verdict trigger\n" +
		"Auditor verdict: FAIL. Root cause reconstructed from stdout for backfill recovery.\n"
	// "last occurrence" wins even with leading bridge/spinner noise.
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

// AC-2 / AC-7 (cycle-187): "build-planner" is a newly backfill-covered phase.
// Its artifact is build-plan.md, headed "# Build Plan" (real on-disk form:
// "# Build Plan — Cycle 121"). With "build-planner" in phaseHeaders, the plan
// must be reconstructed from its stdout.clean.txt.
//
// RED at baseline: no "build-planner" entry in phaseHeaders → (false, nil).
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

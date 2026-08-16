package faillearn

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// novelty_test.go — the near-duplicate gate on the lesson-write seam
// (cycle-1494, `sleep-time-kb-consolidation`). Every case drives the real
// WriteArtifacts and asserts on what actually landed on disk.

// recurringEvent renders the SAME observation at a different cycle — the shape
// that defeats writeIfAbsent's exact-path dedupe, because the lesson id embeds
// the cycle number.
func recurringEvent(cycle int) FailureEvent {
	return FailureEvent{
		Cycle:          cycle,
		FailedPhase:    "build",
		Scope:          ScopePhase,
		Classification: "cycle-mid-execution-fail",
		Verdict:        "FAIL",
		Summary:        "the build phase halted because the contract gate blocked the deliverable for the second consecutive re-dispatch",
		Defects:        []string{"contract-gate-block"},
		Now:            time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC),
	}
}

// unrelatedEvent is a materially different failure: different phase,
// classification, defect and summary vocabulary.
func unrelatedEvent(cycle int) FailureEvent {
	return FailureEvent{
		Cycle:          cycle,
		FailedPhase:    "ship",
		Scope:          ScopePhase,
		Classification: "quota-exhausted",
		Verdict:        "FAIL",
		Summary:        "the ship phase aborted when the provider returned a quota exhaustion response and no fallback CLI family was reachable",
		Defects:        []string{"quota-exhausted"},
		Now:            time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC),
	}
}

func countYAML(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read lessons dir: %v", err)
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".yaml" {
			n++
		}
	}
	return n
}

// TestWriteArtifacts_NoveltyGateSuppressesRecurringObservation is the inbox
// item's literal regression: the same observation written twice leaves ONE
// lesson on disk, even though the two ids differ by cycle number.
func TestWriteArtifacts_NoveltyGateSuppressesRecurringObservation(t *testing.T) {
	lessonsDir := t.TempDir()

	if err := WriteArtifacts(recurringEvent(1494), t.TempDir(), lessonsDir); err != nil {
		t.Fatalf("first WriteArtifacts: %v", err)
	}
	if n := countYAML(t, lessonsDir); n != 1 {
		t.Fatalf("after the first write the corpus holds %d lesson(s), want 1", n)
	}
	if err := WriteArtifacts(recurringEvent(1495), t.TempDir(), lessonsDir); err != nil {
		t.Fatalf("a suppressed near-duplicate must not be an error: %v", err)
	}
	if n := countYAML(t, lessonsDir); n != 1 {
		t.Errorf("corpus holds %d lessons after writing the same observation twice, want 1", n)
	}
}

// TestWriteArtifacts_NoveltyGateStillWritesRetrospective pins that suppression
// is scoped to the LESSON: the failing cycle's own retrospective is its
// durable failure record and must land regardless.
func TestWriteArtifacts_NoveltyGateStillWritesRetrospective(t *testing.T) {
	lessonsDir := t.TempDir()
	if err := WriteArtifacts(recurringEvent(1494), t.TempDir(), lessonsDir); err != nil {
		t.Fatalf("first WriteArtifacts: %v", err)
	}

	runDir := t.TempDir()
	if err := WriteArtifacts(recurringEvent(1495), runDir, lessonsDir); err != nil {
		t.Fatalf("second WriteArtifacts: %v", err)
	}
	if _, err := os.Stat(filepath.Join(runDir, "retrospective-report.md")); err != nil {
		t.Errorf("the suppressed-lesson cycle lost its retrospective: %v", err)
	}
}

// TestWriteArtifacts_NoveltyGateRetainsDistinctFailure is the negative test
// that keeps the gate honest — a suppress-everything implementation passes the
// duplicate case while destroying the corpus.
func TestWriteArtifacts_NoveltyGateRetainsDistinctFailure(t *testing.T) {
	lessonsDir := t.TempDir()
	if err := WriteArtifacts(recurringEvent(1494), t.TempDir(), lessonsDir); err != nil {
		t.Fatalf("first WriteArtifacts: %v", err)
	}
	if err := WriteArtifacts(unrelatedEvent(1495), t.TempDir(), lessonsDir); err != nil {
		t.Fatalf("distinct WriteArtifacts: %v", err)
	}
	if n := countYAML(t, lessonsDir); n != 2 {
		t.Errorf("corpus holds %d lessons, want 2 — a materially different failure must never be suppressed", n)
	}
}

// TestWriteArtifacts_NoveltyGateIsNonDestructiveOnCorpusRot covers the edge
// case parseLessonFile documents: an unparseable neighbour must neither
// suppress the incoming lesson nor be rewritten or deleted.
func TestWriteArtifacts_NoveltyGateIsNonDestructiveOnCorpusRot(t *testing.T) {
	lessonsDir := t.TempDir()
	rotten := filepath.Join(lessonsDir, "rotten.yaml")
	rottenBytes := []byte("id: [this is: not, valid yaml\n  - broken\n")
	if err := os.WriteFile(rotten, rottenBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := WriteArtifacts(unrelatedEvent(1494), t.TempDir(), lessonsDir); err != nil {
		t.Fatalf("a malformed neighbour must not fail the lesson write: %v", err)
	}
	if n := countYAML(t, lessonsDir); n != 2 {
		t.Errorf("corpus holds %d files, want 2 (the rotten file plus the new lesson)", n)
	}
	after, err := os.ReadFile(rotten)
	if err != nil {
		t.Fatalf("the malformed file was deleted by the write path: %v", err)
	}
	if string(after) != string(rottenBytes) {
		t.Errorf("the malformed file was rewritten (got %q, want %q)", after, rottenBytes)
	}
}

// TestWriteArtifacts_NoveltyThresholdOptionReachesTheGate names
// WithNoveltyThreshold and proves the value REACHES the decision rather than
// being dead config: the SAME pair of events that the default threshold keeps
// as two lessons (TestWriteArtifacts_NoveltyGateRetainsDistinctFailure)
// collapses to one under a deliberately loose operator threshold.
func TestWriteArtifacts_NoveltyThresholdOptionReachesTheGate(t *testing.T) {
	lessonsDir := t.TempDir()

	if err := WriteArtifacts(recurringEvent(1494), t.TempDir(), lessonsDir, WithNoveltyThreshold(0.05)); err != nil {
		t.Fatalf("first WriteArtifacts: %v", err)
	}
	if err := WriteArtifacts(unrelatedEvent(1495), t.TempDir(), lessonsDir, WithNoveltyThreshold(0.05)); err != nil {
		t.Fatalf("second WriteArtifacts: %v", err)
	}
	if n := countYAML(t, lessonsDir); n != 1 {
		t.Errorf("corpus holds %d lessons at threshold=0.05, want 1 — the option never reached the gate (the default keeps these two apart)", n)
	}
}

// TestWriteArtifacts_NoveltyThresholdOptionClampsMalformedValue pins the
// resolver's range: a threshold outside (0,1] must fall back to the built-in,
// never disarm the gate (>1) or suppress every write (<=0).
func TestWriteArtifacts_NoveltyThresholdOptionClampsMalformedValue(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   float64
	}{{"zero", 0}, {"negative", -0.5}, {"above-one", 1.5}} {
		var cfg writeConfig
		WithNoveltyThreshold(tc.in)(&cfg)
		if got := cfg.resolvedNoveltyThreshold(); got != defaultNoveltyThreshold {
			t.Errorf("%s: resolvedNoveltyThreshold() = %v, want %v", tc.name, got, defaultNoveltyThreshold)
		}

		lessonsDir := t.TempDir()
		if err := WriteArtifacts(recurringEvent(1494), t.TempDir(), lessonsDir, WithNoveltyThreshold(tc.in)); err != nil {
			t.Fatalf("%s: first WriteArtifacts: %v", tc.name, err)
		}
		if err := WriteArtifacts(unrelatedEvent(1495), t.TempDir(), lessonsDir, WithNoveltyThreshold(tc.in)); err != nil {
			t.Fatalf("%s: distinct WriteArtifacts: %v", tc.name, err)
		}
		if n := countYAML(t, lessonsDir); n != 2 {
			t.Errorf("%s: a malformed threshold suppressed a distinct failure: corpus holds %d, want 2", tc.name, n)
		}
	}
}

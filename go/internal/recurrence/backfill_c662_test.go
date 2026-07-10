package recurrence

// backfill_c662_test.go — cycle-662 RED contract for chronicle-s1-recurrence-index
// gap G2 (HISTORICAL BACKFILL). Cycle 661 (395d8b7e) landed the ledger core but
// it starts EMPTY, so Count()==0 for every historical pattern and the recurrence
// signal is dormant. This file pins the backfill scan over
// .evolve/instincts/lessons/*.yaml that seeds the ledger with the 267-lesson
// history.
//
// Builder contract (do NOT modify this file — implement production code):
//
//	func BackfillFromLessons(lessonsDir string, pol EscalationPolicy) (*Ledger, []string, error)
//	    - scans lessonsDir/*.yaml, tolerating BOTH lesson shapes:
//        deterministic-floor (one-element list, pattern==errorCategory) AND the
//        richer LLM shape (description/governingVariable/preventiveAction blocks);
//	    - records each (pattern, cycle) closure into a fresh ledger, cycle parsed
//	      from the `cycle-<N>-` id prefix; recurrence count == distinct cycles;
//	    - a malformed/unparseable file is SKIPPED (its basename returned in the
//	      second result) and never aborts the scan — err is non-nil only for a
//	      fatal I/O error (dir unreadable), not per-file parse failures.
//
// RED today: BackfillFromLessons does not exist, so package recurrence fails to
// compile and none of these tests run. GREEN once Builder adds backfill.go.

import (
	"os"
	"path/filepath"
	"testing"
)

// writeLesson writes a lesson YAML file (name without extension) into dir.
func writeLesson(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write lesson %s: %v", name, err)
	}
}

// llmLesson renders the richer LLM lesson shape (distinct pattern, non-echo
// errorCategory, prose fields) for a given id/pattern.
func llmLesson(id, pattern string) string {
	return "- id: " + id + "\n" +
		"  pattern: \"" + pattern + "\"\n" +
		"  description: >\n    A rich multi-line narrative describing the defect,\n    authored by the LLM retrospective.\n" +
		"  confidence: 0.95\n" +
		"  type: \"anti-pattern\"\n" +
		"  category: \"double-loop\"\n" +
		"  governingVariable: >\n    The variable to change so the class stops recurring.\n" +
		"  preventiveAction: >\n    A concrete coupled change set.\n" +
		"  failureContext:\n    failedStep: build\n    errorCategory: cycle-mid-execution-fail\n    auditVerdict: FAIL\n"
}

// floorLesson renders the deterministic-floor shape: pattern == errorCategory
// (the echo that marks it Generic), minimal fields.
func floorLesson(id, pattern string) string {
	return "- id: " + id + "\n" +
		"  pattern: " + pattern + "\n" +
		"  description: 'deterministic fallback lesson'\n" +
		"  confidence: 0.5\n" +
		"  type: failure-lesson\n" +
		"  category: episodic\n" +
		"  failureContext:\n    failedStep: retro\n    errorCategory: " + pattern + "\n    auditVerdict: RESET\n"
}

// TestC662_BackfillCountsRecurrenceAcrossLessonShapes — AC2a. The parser must
// tolerate BOTH lesson shapes and accumulate a per-pattern recurrence count
// across distinct cycles. Two LLM-shape files sharing one pattern (cycles 100,
// 101) must count 2; a floor-shape file (cycle 102) must also parse and count 1.
func TestC662_BackfillCountsRecurrenceAcrossLessonShapes(t *testing.T) {
	dir := t.TempDir()
	writeLesson(t, dir, "cycle-100-alpha", llmLesson("cycle-100-alpha", "specific-defect-alpha"))
	writeLesson(t, dir, "cycle-101-beta", llmLesson("cycle-101-beta", "specific-defect-alpha"))
	writeLesson(t, dir, "cycle-102-reset", floorLesson("cycle-102-reset", "operator-reset"))

	led, skipped, err := BackfillFromLessons(dir, DefaultEscalationPolicy())
	if err != nil {
		t.Fatalf("BackfillFromLessons returned fatal error on valid corpus: %v", err)
	}
	if len(skipped) != 0 {
		t.Errorf("no file is malformed, want zero skipped, got %v", skipped)
	}
	if got := led.Count("specific-defect-alpha"); got != 2 {
		t.Errorf("Count(specific-defect-alpha) = %d, want 2 (recurred across LLM-shape cycles 100,101)", got)
	}
	if got := led.Count("operator-reset"); got != 1 {
		t.Errorf("Count(operator-reset) = %d, want 1 (floor-shape lesson must parse too)", got)
	}
	e, ok := led.Entries["specific-defect-alpha"]
	if !ok {
		t.Fatalf("no entry for specific-defect-alpha")
	}
	if !containsInt(e.Cycles, 100) || !containsInt(e.Cycles, 101) {
		t.Errorf("Cycles = %v, want to contain 100 and 101 (cycle parsed from id prefix)", e.Cycles)
	}
}

// TestC662_BackfillSkipsMalformedYAMLWithoutError — AC2b (negative / edge axis).
// A malformed lesson must be recorded in the SkippedFiles diagnostic and never
// abort the scan; the valid file alongside it must still be counted.
func TestC662_BackfillSkipsMalformedYAMLWithoutError(t *testing.T) {
	dir := t.TempDir()
	writeLesson(t, dir, "cycle-200-ok", llmLesson("cycle-200-ok", "valid-pattern-x"))
	// Not a valid lesson: not a YAML list of entries with a pattern key.
	writeLesson(t, dir, "cycle-201-broken", ": : : [ } garbage\n\t- no structure here")

	led, skipped, err := BackfillFromLessons(dir, DefaultEscalationPolicy())
	if err != nil {
		t.Fatalf("a malformed file must be skipped, not returned as a fatal error: %v", err)
	}
	found := false
	for _, s := range skipped {
		if s == "cycle-201-broken.yaml" {
			found = true
		}
	}
	if !found {
		t.Errorf("skipped diagnostic = %v, want it to contain cycle-201-broken.yaml", skipped)
	}
	if got := led.Count("valid-pattern-x"); got != 1 {
		t.Errorf("Count(valid-pattern-x) = %d, want 1 (a malformed neighbor must not drop valid counts)", got)
	}
}

// TestC662_BackfillTaskBindingChainCountsAtLeastSix — AC3 (replay acceptance from
// the chronicle plan: the historical task-binding recurrence chain must be
// visible with its real multi-cycle occurrence count). Six same-pattern floor/LLM
// closeouts across the chain's real cycle numbers must count >= 6 — proof the
// backfill surfaces the long-tail chain the advisory channel merely noticed.
func TestC662_BackfillTaskBindingChainCountsAtLeastSix(t *testing.T) {
	dir := t.TempDir()
	const chain = "execution-phase-binds-scout-selected-tasks-not-triage-topn"
	for _, cyc := range []string{"282", "575", "577", "587", "599", "652"} {
		writeLesson(t, dir, "cycle-"+cyc+"-taskbinding", llmLesson("cycle-"+cyc+"-taskbinding", chain))
	}

	led, _, err := BackfillFromLessons(dir, DefaultEscalationPolicy())
	if err != nil {
		t.Fatalf("BackfillFromLessons: %v", err)
	}
	if got := led.Count(chain); got < 6 {
		t.Errorf("Count(%q) = %d, want >= 6 (the historical task-binding chain must be visible)", chain, got)
	}
}

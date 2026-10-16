package recurrence

import (
	"os"
	"path/filepath"
	"testing"
)

func writeLesson(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write lesson %s: %v", name, err)
	}
}

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

func floorLesson(id, pattern string) string {
	return "- id: " + id + "\n" +
		"  pattern: " + pattern + "\n" +
		"  description: 'deterministic fallback lesson'\n" +
		"  confidence: 0.5\n" +
		"  type: failure-lesson\n" +
		"  category: episodic\n" +
		"  failureContext:\n    failedStep: retro\n    errorCategory: " + pattern + "\n    auditVerdict: RESET\n"
}

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

func TestC662_BackfillSkipsMalformedYAMLWithoutError(t *testing.T) {
	dir := t.TempDir()
	writeLesson(t, dir, "cycle-200-ok", llmLesson("cycle-200-ok", "valid-pattern-x"))
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

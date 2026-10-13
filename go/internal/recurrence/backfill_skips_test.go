package recurrence

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestBackfillFromLessons_RejectsAMalformedGlobPattern(t *testing.T) {
	t.Parallel()

	led, skipped, err := BackfillFromLessons("[", DefaultEscalationPolicy())

	if err == nil || led != nil || skipped != nil {
		t.Errorf("BackfillFromLessons = %v, %v, %v, want a fatal glob error", led, skipped, err)
	}
}

func TestBackfillFromLessons_SkipsWhatItCannotCount(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "unreadable.yaml"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeLesson(t, dir, "empty-pattern", "- id: cycle-1-a\n  pattern: \"  \"\n")
	writeLesson(t, dir, "no-cycle", "- id: lesson-a\n  pattern: specific-defect\n")
	writeLesson(t, dir, "counted", "- id: cycle-7-a\n  pattern: specific-defect\n")

	led, skipped, err := BackfillFromLessons(dir, DefaultEscalationPolicy())

	if err != nil {
		t.Fatalf("BackfillFromLessons: %v", err)
	}
	want := []string{"empty-pattern.yaml", "no-cycle.yaml", "unreadable.yaml"}
	if got := slices.Sorted(slices.Values(skipped)); !slices.Equal(got, want) {
		t.Errorf("skipped = %v, want %v", got, want)
	}
	if len(led.Entries) != 1 || led.Entries["specific-defect"] == nil {
		t.Errorf("entries = %v, want only the counted lesson's pattern", led.Entries)
	}
}

func TestCycleFromID_RejectsIdsWithoutAnIntegerCycle(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"lesson-a", "cycle-5", "cycle--a", "cycle-x-a"} {
		if n, ok := cycleFromID(id); ok {
			t.Errorf("cycleFromID(%q) = %d, true, want false", id, n)
		}
	}
	if n, ok := cycleFromID("cycle-12-a"); !ok || n != 12 {
		t.Errorf("cycleFromID(cycle-12-a) = %d, %v, want 12, true", n, ok)
	}
}

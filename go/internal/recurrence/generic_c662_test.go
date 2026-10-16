package recurrence

import "testing"

func TestC662_MarksClassificationEchoPatternsGeneric(t *testing.T) {
	if !IsGeneric("operator-reset", "operator-reset") {
		t.Errorf("IsGeneric(operator-reset echoing its errorCategory) = false, want true")
	}
	if !IsGeneric("loop-fatal", "loop-fatal") {
		t.Errorf("IsGeneric(loop-fatal) = false, want true (denylist + echo)")
	}
	if IsGeneric("builder-out-of-lane-build-ships-red", "cycle-mid-execution-fail") {
		t.Errorf("IsGeneric(specific defect != errorCategory) = true, want false (not noise)")
	}

	dir := t.TempDir()
	writeLesson(t, dir, "cycle-410-reset", floorLesson("cycle-410-reset", "operator-reset"))
	writeLesson(t, dir, "cycle-411-defect", llmLesson("cycle-411-defect", "specific-defect-beta"))

	led, _, err := BackfillFromLessons(dir, DefaultEscalationPolicy())
	if err != nil {
		t.Fatalf("BackfillFromLessons: %v", err)
	}
	if !led.IsGenericPattern("operator-reset") {
		t.Errorf("IsGenericPattern(operator-reset) = false, want true after backfill")
	}
	if led.IsGenericPattern("specific-defect-beta") {
		t.Errorf("IsGenericPattern(specific-defect-beta) = true, want false (specific defect)")
	}
	if e := led.Entries["operator-reset"]; e == nil || !e.Generic {
		t.Errorf("Entry(operator-reset).Generic must be true after backfill; entry=%+v", e)
	}
	if e := led.Entries["specific-defect-beta"]; e == nil || e.Generic {
		t.Errorf("Entry(specific-defect-beta).Generic must be false after backfill; entry=%+v", e)
	}
	if led.IsGenericPattern("never-recorded") {
		t.Errorf("IsGenericPattern(unseen) = true, want false")
	}
}

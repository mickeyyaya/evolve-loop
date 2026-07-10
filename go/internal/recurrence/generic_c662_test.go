package recurrence

// generic_c662_test.go — cycle-662 RED contract for chronicle-s1-recurrence-index
// gap G3 (GENERIC CLASSIFICATION de-noise). operator-reset (x96) + loop-fatal
// (x62) are 59% of the 267-lesson corpus; without a generic filter they dominate
// the counts and spam escalation (plan failure-mode 2). This file pins the
// per-pattern Generic classification.
//
// Builder contract (do NOT modify this file — implement production code):
//
//	Entry.Generic bool                                   // json:"generic,omitempty"
//	func IsGeneric(pattern, errorCategory string) bool   // denylist + echo rule
//	func (l *Ledger) IsGenericPattern(pattern string) bool
//
//	Generic == true when the pattern is classification-vocabulary noise: either it
//	is on the denylist (operator-reset, loop-fatal, ...) OR it verbatim echoes the
//	lesson's failureContext.errorCategory. Backfill sets Entry.Generic via
//	IsGeneric; IsGenericPattern reads the stored flag (false for unseen patterns).
//
// RED today: Entry.Generic / IsGeneric / IsGenericPattern do not exist, so
// package recurrence fails to compile. GREEN once Builder adds generic.go.

import "testing"

// TestC662_MarksClassificationEchoPatternsGeneric — AC4a. The echo rule and the
// denylist both mark a pattern generic; a distinct specific-defect pattern
// (pattern != errorCategory) stays non-generic. Backfill must persist the flag
// per entry and IsGenericPattern must reflect it.
func TestC662_MarksClassificationEchoPatternsGeneric(t *testing.T) {
	// Pure classifier: echo rule (pattern == errorCategory).
	if !IsGeneric("operator-reset", "operator-reset") {
		t.Errorf("IsGeneric(operator-reset echoing its errorCategory) = false, want true")
	}
	if !IsGeneric("loop-fatal", "loop-fatal") {
		t.Errorf("IsGeneric(loop-fatal) = false, want true (denylist + echo)")
	}
	// A specific LLM defect whose pattern differs from its errorCategory is NOT
	// generic — it is exactly the signal escalation must keep.
	if IsGeneric("builder-out-of-lane-build-ships-red", "cycle-mid-execution-fail") {
		t.Errorf("IsGeneric(specific defect != errorCategory) = true, want false (not noise)")
	}

	// Backfill must persist Generic per entry and expose it via IsGenericPattern.
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
	// Semantic anti-no-op: IsGenericPattern for an unseen pattern is false.
	if led.IsGenericPattern("never-recorded") {
		t.Errorf("IsGenericPattern(unseen) = true, want false")
	}
}

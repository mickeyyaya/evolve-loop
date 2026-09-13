package cyclestate

import "testing"

// ADR-0103 unit 03b: the supervisor's default class for a phase that failed
// mid-cycle with no self-report has ONE spelling — the failure-learning engine
// projects it into the FailedRecord and the lesson event, recurrence's generic
// denylist names it. The literal is the on-disk contract of every legacy record.
func TestClassificationMidExecutionFail_IsTheLegacyDefaultSpelling(t *testing.T) {
	if ClassificationMidExecutionFail != "cycle-mid-execution-fail" {
		t.Fatalf("the default class is the legacy on-disk spelling, got %q", ClassificationMidExecutionFail)
	}
}

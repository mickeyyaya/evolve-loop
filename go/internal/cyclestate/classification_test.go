package cyclestate

import "testing"

func TestClassificationMidExecutionFail_IsTheLegacyDefaultSpelling(t *testing.T) {
	if ClassificationMidExecutionFail != "cycle-mid-execution-fail" {
		t.Fatalf("the default class is the legacy on-disk spelling, got %q", ClassificationMidExecutionFail)
	}
}

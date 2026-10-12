package quotastate

import (
	"testing"
	"time"
)

func TestCompileLabels_NoLabelsIsRefusedBeforeAnyPatternIsBuilt(t *testing.T) {
	for _, labels := range [][]WindowLabel{nil, {}} {
		patterns, err := compileLabels(labels)
		if err == nil || patterns != nil || err.Error() != "labels: needs at least one label" {
			t.Fatalf("compileLabels(%v) = %v, %v", labels, patterns, err)
		}
	}
}

func TestWindowReaderSettled_DraftsWithoutAValueAreDroppedAndTheRestKeepTheirOrder(t *testing.T) {
	r := &windowReader{drafts: []windowDraft{
		{UsageWindow: UsageWindow{Scope: "first", PercentUsed: 40}, hasValue: true},
		{UsageWindow: UsageWindow{Scope: "reset-only", ResetsText: "in 2h"}, hasReset: true},
		{UsageWindow: UsageWindow{Scope: "last", PercentUsed: 100}, hasValue: true},
	}}

	got := r.settled(WindowSpec{}, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))

	if len(got) != 2 || got[0].Scope != "first" || got[1].Scope != "last" {
		t.Fatalf("settled = %+v", got)
	}
	if got[0].Exhausted || !got[1].Exhausted {
		t.Errorf("exhaustion follows the used percentage: %+v", got)
	}
	if empty := (&windowReader{}).settled(WindowSpec{}, time.Time{}); len(empty) != 0 {
		t.Errorf("no drafts, no windows: %+v", empty)
	}
}

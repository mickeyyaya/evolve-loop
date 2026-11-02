package cyclespend

import (
	"math"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
)

var cycleStart = time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)

func minuteMark(m int) time.Time { return cycleStart.Add(time.Duration(m) * time.Minute) }

func callEndingAt(endMinute, durationMinutes int) llmcalls.Record {
	d := int64(durationMinutes) * 60_000
	return llmcalls.Record{DurationMS: &d, EndedAt: minuteMark(endMinute).Format(time.RFC3339Nano), UsageStatus: llmcalls.UsageMeasured}
}

func wantMinutes(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	if got == nil || math.Abs(*got-want) > 1e-9 {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func TestVerdictOf_AFailedCycleWithAFloorRejectionAtMinute30AndTwentyMoreMinutesHasFailPoint30Waste20Ratio06(t *testing.T) {
	t.Parallel()
	calls := []llmcalls.Record{callEndingAt(10, 10), callEndingAt(30, 20), callEndingAt(50, 20)}

	got := VerdictOf(calls, minuteMark(30), false)

	if got.LLMMinutesToVerdict != 50 {
		t.Errorf("LLMMinutesToVerdict = %v, want 50", got.LLMMinutesToVerdict)
	}
	wantMinutes(t, "FailPointMinutes", got.FailPointMinutes, 30)
	wantMinutes(t, "WasteMinutes", got.WasteMinutes, 20)
	wantMinutes(t, "FailFastRatio", got.FailFastRatio, 0.6)
}

func TestVerdictOf_AShippedCycleHasMinutesToShipAndNoFailPoint(t *testing.T) {
	t.Parallel()
	calls := []llmcalls.Record{callEndingAt(10, 10), callEndingAt(30, 20), callEndingAt(50, 20)}

	got := VerdictOf(calls, minuteMark(30), true)

	if got.LLMMinutesToVerdict != 50 {
		t.Errorf("LLMMinutesToVerdict = %v, want 50", got.LLMMinutesToVerdict)
	}
	if got.FailPointMinutes != nil || got.WasteMinutes != nil || got.FailFastRatio != nil {
		t.Errorf("verdict = %+v, want no fail point, waste or ratio for a cycle that shipped", got)
	}
}

func TestVerdictOf_AFailedCycleWithNoBlockingSignalHasNoFailPoint(t *testing.T) {
	t.Parallel()
	calls := []llmcalls.Record{callEndingAt(10, 10), callEndingAt(40, 30)}

	got := VerdictOf(calls, time.Time{}, false)

	if got.LLMMinutesToVerdict != 40 || got.FailPointMinutes != nil || got.WasteMinutes != nil || got.FailFastRatio != nil {
		t.Errorf("verdict = %+v, want 40 minutes to verdict and no fail point", got)
	}
}

func TestVerdictOf_ACallThatEndsAtTheFailPointCountsBeforeIt(t *testing.T) {
	t.Parallel()
	calls := []llmcalls.Record{callEndingAt(30, 30), callEndingAt(31, 1)}

	got := VerdictOf(calls, minuteMark(30), false)

	wantMinutes(t, "FailPointMinutes", got.FailPointMinutes, 30)
	wantMinutes(t, "WasteMinutes", got.WasteMinutes, 1)
}

func TestVerdictOf_ACycleThatStopsAtTheFailPointWastesNothingAndHasRatio1(t *testing.T) {
	t.Parallel()
	calls := []llmcalls.Record{callEndingAt(30, 30)}

	got := VerdictOf(calls, minuteMark(30), false)

	wantMinutes(t, "WasteMinutes", got.WasteMinutes, 0)
	wantMinutes(t, "FailFastRatio", got.FailFastRatio, 1)
}

func TestVerdictOf_ACycleWithNoMeasuredTimeHasNoRatioInsteadOfNaN(t *testing.T) {
	t.Parallel()
	for name, calls := range map[string][]llmcalls.Record{
		"no calls":   nil,
		"unmeasured": {{EndedAt: minuteMark(5).Format(time.RFC3339Nano), UsageStatus: llmcalls.UsageUnavailable}},
	} {
		got := VerdictOf(calls, minuteMark(30), false)
		if got.LLMMinutesToVerdict != 0 || got.FailFastRatio != nil {
			t.Errorf("%s: verdict = %+v, want 0 minutes and no ratio", name, got)
		}
	}
}

func TestVerdictOf_ACallWithNoEndTimeIsPlacedByItsTimestamp(t *testing.T) {
	t.Parallel()
	d := int64(10 * 60_000)
	early := llmcalls.Record{DurationMS: &d, TS: minuteMark(20).Format(time.RFC3339Nano)}
	late := llmcalls.Record{DurationMS: &d, TS: minuteMark(45).Format(time.RFC3339Nano)}

	got := VerdictOf([]llmcalls.Record{early, late}, minuteMark(30), false)

	wantMinutes(t, "FailPointMinutes", got.FailPointMinutes, 10)
	wantMinutes(t, "WasteMinutes", got.WasteMinutes, 10)
}

func TestVerdictOf_ACallWithNoTimeAtAllCountsToTheVerdictButNeverAsWaste(t *testing.T) {
	t.Parallel()
	d := int64(10 * 60_000)

	got := VerdictOf([]llmcalls.Record{{DurationMS: &d}}, minuteMark(30), false)

	if got.LLMMinutesToVerdict != 10 {
		t.Errorf("LLMMinutesToVerdict = %v, want 10", got.LLMMinutesToVerdict)
	}
	wantMinutes(t, "WasteMinutes", got.WasteMinutes, 0)
}

func TestVerdictOf_TheMinutesAreTheSameSumAsSpend(t *testing.T) {
	t.Parallel()
	calls := []llmcalls.Record{callEndingAt(10, 10), callEndingAt(30, 20), callEndingAt(50, 20)}

	if got, want := VerdictOf(calls, minuteMark(30), false).LLMMinutesToVerdict, Sum(calls).LLMMinutes(); got != want {
		t.Errorf("LLMMinutesToVerdict = %v, Sum.LLMMinutes = %v; the two must agree", got, want)
	}
}

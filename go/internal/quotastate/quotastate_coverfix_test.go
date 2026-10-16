package quotastate

import (
	"testing"
	"time"
)

func TestParseResetWhen_RejectsAnUnknownMonthPrefix(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
	if got, ok := ParseResetWhen("Foo 5 at 9pm", now); ok || !got.IsZero() {
		t.Fatalf("a word that is no month must not parse a reset: got %v ok=%v", got, ok)
	}
}

func TestParseResetWhen_DateMoreThanADayPastRollsToNextYear(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.December, 31, 10, 0, 0, 0, time.UTC)
	got, ok := ParseResetWhen("Jan 5 at 9pm", now)
	want := time.Date(2027, time.January, 5, 21, 0, 0, 0, time.UTC)
	if !ok || !got.Equal(want) {
		t.Fatalf("a date long past in this year must roll to next year: got %v ok=%v, want %v", got, ok, want)
	}
}

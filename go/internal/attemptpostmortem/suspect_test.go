package attemptpostmortem

import (
	"testing"
	"time"
)

func TestFindSuspect_PicksTheLastCommandThatMatchesAnyRule(t *testing.T) {
	end := time.Date(2026, 10, 9, 9, 21, 8, 0, time.UTC)
	window := 30 * time.Second
	at := func(before time.Duration) time.Time { return end.Add(-before) }
	cases := []struct {
		name       string
		commands   []Command
		wantText   string
		wantReason SuspectReason
	}{
		{"no commands", nil, "", ""},
		{"all old and ok", []Command{{Text: "a", StartedAt: at(time.Hour), Status: StatusOK}}, "", ""},
		{"a command with no result", []Command{{Text: "a", StartedAt: at(time.Hour), Status: StatusNoResult}, {Text: "b", StartedAt: at(2 * time.Hour), Status: StatusOK}}, "a", ReasonNoResult},
		{"a signal exit", []Command{{Text: "a", StartedAt: at(time.Hour), Status: StatusSignal, ExitCode: 137}}, "a", ReasonSignalExit},
		{"a plain error is not a suspect", []Command{{Text: "a", StartedAt: at(time.Hour), Status: StatusError, ExitCode: 1}}, "", ""},
		{"inside the window", []Command{{Text: "a", StartedAt: at(time.Hour), Status: StatusSignal}, {Text: "b", StartedAt: at(window - time.Millisecond), Status: StatusOK}}, "b", ReasonEndWindow},
		{"on the window edge", []Command{{Text: "a", StartedAt: at(window), Status: StatusOK}}, "a", ReasonEndWindow},
		{"just outside the window", []Command{{Text: "a", StartedAt: at(window + time.Millisecond), Status: StatusOK}}, "", ""},
		{"the latest match wins", []Command{{Text: "a", StartedAt: at(time.Hour), Status: StatusNoResult}, {Text: "b", StartedAt: at(50 * time.Minute), Status: StatusSignal}, {Text: "c", StartedAt: at(40 * time.Minute), Status: StatusOK}}, "b", ReasonSignalExit},
		{"no result outranks the window", []Command{{Text: "a", StartedAt: at(time.Second), Status: StatusNoResult}}, "a", ReasonNoResult},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := findSuspect(tc.commands, end, window)
			if tc.wantText == "" {
				if got != nil {
					t.Fatalf("findSuspect = %+v, want nil", got)
				}
				return
			}
			if got == nil || got.Command.Text != tc.wantText || got.Reason != tc.wantReason {
				t.Fatalf("findSuspect = %+v, want %q by %q", got, tc.wantText, tc.wantReason)
			}
		})
	}
}

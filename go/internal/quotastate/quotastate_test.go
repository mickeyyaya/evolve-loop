package quotastate

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// refNow anchors relative reset times deterministically: 2026-07-03 in
// Asia/Taipei (the tz the golden pane was captured in), before both the 4:10pm
// session reset and the Jul 5 weekly reset.
func refNow(t *testing.T) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		loc = time.FixedZone("Asia/Taipei", 8*3600)
	}
	return time.Date(2026, time.July, 3, 12, 0, 0, 0, loc)
}

func TestReadWindows_TheJulyClaudeGoldenKeepsItsBucketsThroughStatesOf(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "claude_usage.txt"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	now := refNow(t)

	states := StatesOf(ReadWindows(claudeWindowSpec, string(raw), now), now)

	if len(states) != 1 || states[0].Family != "claude" || states[0].Source != SourceProbed || states[0].Exhausted {
		t.Fatalf("states = %+v; want one probed claude state that its drained Fable window does not exhaust", states)
	}
	byName := map[string]Bucket{}
	for _, b := range states[0].Buckets {
		byName[b.Name] = b
	}
	for name, want := range map[string]struct {
		used  float64
		reset time.Time
	}{
		"session": {0.27, time.Date(2026, time.July, 3, 16, 10, 0, 0, now.Location())},
		"week":    {0.66, time.Date(2026, time.July, 5, 21, 0, 0, 0, now.Location())},
	} {
		b, ok := byName[name]
		if !ok || !approx(b.UsedFraction, want.used) || !b.ResetAt.Equal(want.reset) {
			t.Errorf("bucket %s = %+v (present=%v); want used %v, reset %v", name, b, ok, want.used, want.reset)
		}
	}
	if got, ok := states[0].TightestRemaining("session", "week"); !ok || !approx(got, 0.34) {
		t.Errorf("TightestRemaining(session,week) = %v,%v, want 0.34,true (weekly binds)", got, ok)
	}
	if _, perModel := byName["week:Fable"]; perModel || len(byName) != 2 {
		t.Errorf("buckets = %v; a family's quota state holds only its family-scoped windows", byName)
	}
	if got := byName["session"].RemainingFraction(); !approx(got, 0.73) {
		t.Errorf("session RemainingFraction() = %v, want 0.73", got)
	}
	for _, q := range states {
		if q.Source == SourceUnknown || q.Source != SourceProbed {
			t.Errorf("%s state Source = %q; a state built from read windows is always probed, never %q", q.Family, q.Source, SourceUnknown)
		}
	}
}

func TestStatesOf_NoWindowsIsNoState(t *testing.T) {
	for name, pane := range map[string]string{
		"empty":     "",
		"blank":     "\n\n   \n",
		"unrelated": "codex v1.2\ntype /help for commands\n> ",
	} {
		if got := StatesOf(ReadWindows(claudeWindowSpec, pane, refNow(t)), refNow(t)); len(got) != 0 {
			t.Errorf("%s: states = %+v, want none (no fabricated cap)", name, got)
		}
	}
	var none QuotaState
	if _, ok := none.TightestRemaining(); ok {
		t.Errorf("TightestRemaining on a state with no buckets: ok=true, want false")
	}
}

// TestParseResetWhen pins the two reset formats claude emits.
func TestParseResetWhen(t *testing.T) {
	now := refNow(t)
	cases := []struct {
		in   string
		want time.Time
	}{
		{"4:10pm", time.Date(2026, 7, 3, 16, 10, 0, 0, now.Location())},
		{"9pm", time.Date(2026, 7, 3, 21, 0, 0, 0, now.Location())},
		{"Jul 5 at 9pm", time.Date(2026, 7, 5, 21, 0, 0, 0, now.Location())},
		{"Jul 5 at 9:30pm", time.Date(2026, 7, 5, 21, 30, 0, 0, now.Location())},
		{"11am", time.Date(2026, 7, 4, 11, 0, 0, 0, now.Location())}, // already past noon → tomorrow
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := ParseResetWhen(tc.in, now)
			if !ok || !got.Equal(tc.want) {
				t.Errorf("ParseResetWhen(%q) = %v,%v, want %v,true", tc.in, got, ok, tc.want)
			}
		})
	}
	if _, ok := ParseResetWhen("whenever", now); ok {
		t.Errorf("ParseResetWhen(garbage) ok=true, want false")
	}
}

func approx(a, b float64) bool {
	d := a - b
	return d < 0.001 && d > -0.001
}

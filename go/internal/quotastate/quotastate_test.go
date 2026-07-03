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

// TestParse_ClaudeGolden parses the REAL captured claude /usage pane (testdata/
// claude_usage.txt) and asserts every bucket + used% + reset. This is the whole
// design premise: claude reports numeric per-window usage, so the budget can be
// real. Gaming kills: hard-coding the 3 buckets fails when the golden changes;
// returning Source=unknown fails the numeric assertions.
func TestParse_ClaudeGolden(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "claude_usage.txt"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	now := refNow(t)
	var q QuotaState = Parse("claude", string(raw), now)

	if q.Source != SourceProbed {
		t.Fatalf("Source = %q, want %q (numeric buckets parsed)", q.Source, SourceProbed)
	}
	if len(q.Buckets) != 3 {
		t.Fatalf("got %d buckets, want 3 (session, week, week:Fable); buckets=%+v", len(q.Buckets), q.Buckets)
	}
	if !q.Exhausted {
		t.Errorf("Exhausted = false, want true (the Fable week bucket is at 100%%)")
	}

	byName := map[string]Bucket{}
	for _, b := range q.Buckets {
		byName[b.Name] = b
	}

	// session: 27% used → 73% remaining, resets 4:10pm today (still ahead of noon).
	sess, ok := byName["session"]
	if !ok {
		t.Fatalf("missing 'session' bucket; got %v", byName)
	}
	if !approx(sess.UsedFraction, 0.27) {
		t.Errorf("session UsedFraction = %v, want 0.27", sess.UsedFraction)
	}
	if !approx(sess.RemainingFraction(), 0.73) {
		t.Errorf("session RemainingFraction() = %v, want 0.73", sess.RemainingFraction())
	}
	wantSessReset := time.Date(2026, time.July, 3, 16, 10, 0, 0, now.Location())
	if !sess.ResetAt.Equal(wantSessReset) {
		t.Errorf("session ResetAt = %v, want %v (4:10pm today)", sess.ResetAt, wantSessReset)
	}

	// week (all models): 66% used → the tightest APPLICABLE cap for opus/sonnet.
	wk, ok := byName["week"]
	if !ok {
		t.Fatalf("missing 'week' bucket; got %v", byName)
	}
	if !approx(wk.UsedFraction, 0.66) {
		t.Errorf("week UsedFraction = %v, want 0.66", wk.UsedFraction)
	}
	wantWkReset := time.Date(2026, time.July, 5, 21, 0, 0, 0, now.Location())
	if !wk.ResetAt.Equal(wantWkReset) {
		t.Errorf("week ResetAt = %v, want %v (Jul 5 at 9pm)", wk.ResetAt, wantWkReset)
	}

	// week:Fable — the per-model window, at 100% (why deep moved off fable).
	fab, ok := byName["week:Fable"]
	if !ok {
		t.Fatalf("missing 'week:Fable' bucket; got %v", byName)
	}
	if !approx(fab.UsedFraction, 1.0) {
		t.Errorf("week:Fable UsedFraction = %v, want 1.0", fab.UsedFraction)
	}

	// TightestRemaining over the general windows (session+week) = the weekly
	// 34%, not the roomier 73% session — the binding constraint the budget uses.
	got, ok := q.TightestRemaining("session", "week")
	if !ok || !approx(got, 0.34) {
		t.Errorf("TightestRemaining(session,week) = %v,%v, want 0.34,true (weekly binds)", got, ok)
	}
}

// TestParse_UnknownPane: a pane with no recognizable bucket (an unsupported CLI
// or a garbled capture) yields Source=unknown with NO fabricated buckets — the
// budget must degrade to the min_lanes floor, never invent a cap.
func TestParse_UnknownPane(t *testing.T) {
	for name, pane := range map[string]string{
		"empty":     "",
		"blank":     "\n\n   \n",
		"unrelated": "codex v1.2\ntype /help for commands\n> ",
	} {
		t.Run(name, func(t *testing.T) {
			q := Parse("codex", pane, refNow(t))
			if q.Source != SourceUnknown {
				t.Errorf("Source = %q, want %q", q.Source, SourceUnknown)
			}
			if len(q.Buckets) != 0 {
				t.Errorf("got %d buckets, want 0 (no fabricated cap): %+v", len(q.Buckets), q.Buckets)
			}
			if _, ok := q.TightestRemaining(); ok {
				t.Errorf("TightestRemaining ok=true on unknown pane, want false")
			}
		})
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

package bridge

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

const lateEnterNudge = "Please write the deliverable to /ws/build-report.md to complete the phase."

type virtualClock struct {
	mu  sync.Mutex
	now time.Duration
}

func (c *virtualClock) advance(d time.Duration) {
	c.mu.Lock()
	c.now += d
	c.mu.Unlock()
}

func (c *virtualClock) read() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

type lateEnterTmux struct {
	*fakeTmux
	clock         *virtualClock
	ignoreUntil   time.Duration
	renderLatency time.Duration
	mu            sync.Mutex
	acceptedAt    time.Duration
	accepted      bool
	enterAt       []time.Duration
}

func (l *lateEnterTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if enter {
		now := l.clock.read()
		l.enterAt = append(l.enterAt, now)
		if now >= l.ignoreUntil && !l.accepted {
			l.accepted = true
			l.acceptedAt = now
		}
	}
	return l.fakeTmux.SendKeys(ctx, session, keys, enter)
}

func (l *lateEnterTmux) CapturePane(context.Context, string, int) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.accepted && l.clock.read() >= l.acceptedAt+l.renderLatency {
		return "● earlier scrollback\n\n" + tmuxPromptMarkerDefault + " ", nil
	}
	return parkedPane(lateEnterNudge), nil
}

func (l *lateEnterTmux) enters() []time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]time.Duration(nil), l.enterAt...)
}

func newLateEnterSeat(ignoreUntil time.Duration) (*lateEnterTmux, Deps, *bytes.Buffer) {
	clock := &virtualClock{}
	tm := &lateEnterTmux{fakeTmux: &fakeTmux{}, clock: clock, ignoreUntil: ignoreUntil}
	var stderr bytes.Buffer
	deps := submitVerifyDeps(tm, &stderr)
	deps.Sleep = clock.advance
	return tm, deps, &stderr
}

func runLateEnterVerify(deps Deps) submitVerifyOutcome {
	lp := tmuxLaunch{name: "claude-tmux", session: "s",
		promptMarker: tmuxPromptMarkerDefault, inputLineMarker: tmuxPromptMarkerDefault}
	return verifySubmitted(context.Background(), deps, lp, "[claude-tmux]", "prompt",
		parkedPane(lateEnterNudge), lateEnterNudge)
}

func TestVerifySubmitted_EnterIgnoredFor20sThenAcceptedEndsSubmittedAfterResend(t *testing.T) {
	t.Parallel()
	tm, deps, stderr := newLateEnterSeat(20 * time.Second)
	got := runLateEnterVerify(deps)
	if got.Result != interaction.ResultSubmittedAfterResend {
		t.Fatalf("result = %q, want %q (enters at %v); stderr=%s", got.Result, interaction.ResultSubmittedAfterResend, tm.enters(), stderr.String())
	}
	if got.Resends < 1 {
		t.Errorf("Resends = %d, want at least 1", got.Resends)
	}
}

func TestVerifySubmitted_DefaultScheduleResendsAtTwoFiveTenTwentyThirtySeconds(t *testing.T) {
	t.Parallel()
	tm, deps, _ := newLateEnterSeat(time.Hour)
	got := runLateEnterVerify(deps)
	want := []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second, 20 * time.Second, 30 * time.Second}
	assertEnterSchedule(t, tm.enters(), want)
	if got.Result != interaction.ResultSubmitWedged || got.Resends != len(want) {
		t.Errorf("outcome = %+v, want %q after %d resends", got, interaction.ResultSubmitWedged, len(want))
	}
}

func TestVerifySubmitted_ResendScheduleComesFromDeps(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		schedule []time.Duration
	}{
		{"single", []time.Duration{3 * time.Second}},
		{"short", []time.Duration{time.Second, 4 * time.Second}},
		{"long", []time.Duration{time.Second, 3 * time.Second, 9 * time.Second, 27 * time.Second}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tm, deps, _ := newLateEnterSeat(time.Hour)
			deps.SubmitResendSchedule = tc.schedule
			got := runLateEnterVerify(deps)
			assertEnterSchedule(t, tm.enters(), tc.schedule)
			if got.Result != interaction.ResultSubmitWedged || got.Resends != len(tc.schedule) {
				t.Errorf("outcome = %+v, want %q after %d resends", got, interaction.ResultSubmitWedged, len(tc.schedule))
			}
		})
	}
}

func TestVerifySubmitted_ConfiguredScheduleStopsResendingOnceTheEnterIsAccepted(t *testing.T) {
	t.Parallel()
	tm, deps, _ := newLateEnterSeat(7 * time.Second)
	deps.SubmitResendSchedule = []time.Duration{2 * time.Second, 5 * time.Second, 8 * time.Second, 12 * time.Second}
	got := runLateEnterVerify(deps)
	assertEnterSchedule(t, tm.enters(), deps.SubmitResendSchedule[:3])
	if got.Result != interaction.ResultSubmittedAfterResend || got.Resends != 3 {
		t.Errorf("outcome = %+v, want %q after 3 resends", got, interaction.ResultSubmittedAfterResend)
	}
}

func TestVerifySubmitted_FourConcurrentSeatsAllSubmitThreeRoundsInARow(t *testing.T) {
	t.Parallel()
	for round := 1; round <= 3; round++ {
		var wg sync.WaitGroup
		results := make([]string, 4)
		for seat := range results {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, deps, _ := newLateEnterSeat(20 * time.Second)
				results[seat] = runLateEnterVerify(deps).Result
			}()
		}
		wg.Wait()
		for seat, r := range results {
			if r != interaction.ResultSubmittedAfterResend {
				t.Errorf("round %d seat %d result = %q, want %q", round, seat, r, interaction.ResultSubmittedAfterResend)
			}
		}
	}
}

func newRenderingSeat(ignoreUntil, renderLatency time.Duration) (*lateEnterTmux, *virtualClock, Deps) {
	tm, deps, _ := newLateEnterSeat(ignoreUntil)
	tm.renderLatency = renderLatency
	return tm, tm.clock, deps
}

func TestVerifySubmitted_FinalScheduledEnterIsObservedBeforeGivingUp(t *testing.T) {
	t.Parallel()
	for _, latency := range []time.Duration{100 * time.Millisecond, 250 * time.Millisecond, 400 * time.Millisecond} {
		t.Run(latency.String(), func(t *testing.T) {
			t.Parallel()
			tm, _, deps := newRenderingSeat(30*time.Second, latency)
			got := runLateEnterVerify(deps)
			want := []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second, 20 * time.Second, 30 * time.Second}
			assertEnterSchedule(t, tm.enters(), want)
			if got.Result != interaction.ResultSubmittedAfterResend || got.Resends != len(want) {
				t.Errorf("outcome = %+v, want %q after %d resends: the pane renders the submit %v after the last Enter",
					got, interaction.ResultSubmittedAfterResend, len(want), latency)
			}
		})
	}
}

func TestVerifySubmitted_AcceptedEnterIsObservedBeforeTheNextEnterIsSent(t *testing.T) {
	t.Parallel()
	tm, _, deps := newRenderingSeat(20*time.Second, 250*time.Millisecond)
	got := runLateEnterVerify(deps)
	want := []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second, 20 * time.Second}
	assertEnterSchedule(t, tm.enters(), want)
	if got.Result != interaction.ResultSubmittedAfterResend || got.Resends != len(want) {
		t.Errorf("outcome = %+v, want %q after %d resends", got, interaction.ResultSubmittedAfterResend, len(want))
	}
}

func TestVerifySubmitted_NeverAcceptedPaneIsObservedAfterTheLastEnterThenWedges(t *testing.T) {
	t.Parallel()
	_, clock, deps := newRenderingSeat(time.Hour, 250*time.Millisecond)
	got := runLateEnterVerify(deps)
	if got.Result != interaction.ResultSubmitWedged || got.Resends != len(defaultSubmitResendSchedule) {
		t.Fatalf("outcome = %+v, want %q after %d resends", got, interaction.ResultSubmitWedged, len(defaultSubmitResendSchedule))
	}
	lastEnter := defaultSubmitResendSchedule[len(defaultSubmitResendSchedule)-1]
	end := clock.read()
	if end < lastEnter+submitVerifySettle || end > lastEnter+5*time.Second {
		t.Errorf("verification ended at %v, want a final observation window of [%v, 5s] after the last Enter at %v", end, submitVerifySettle, lastEnter)
	}
}

func assertEnterSchedule(t *testing.T, got, want []time.Duration) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("Enter sent %d time(s) at %v, want %d at the deadlines %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] < want[i] || got[i] >= want[i]+time.Second {
			t.Errorf("Enter %d sent at %v, want within [%v, %v)", i+1, got[i], want[i], want[i]+time.Second)
		}
	}
}

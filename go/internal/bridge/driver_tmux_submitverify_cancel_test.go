package bridge

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

type cancelOnSendTmux struct {
	*fakeTmux
	cancel      context.CancelFunc
	cancelAt    int
	cancelAfter time.Duration
	sends       atomic.Int32
}

func (c *cancelOnSendTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	n := int(c.sends.Add(1))
	if n == c.cancelAt {
		if c.cancelAfter == 0 {
			c.cancel()
		} else {
			time.AfterFunc(c.cancelAfter, c.cancel)
		}
	}
	return c.fakeTmux.SendKeys(ctx, session, keys, enter)
}

func withBackoff(t *testing.T, steps [submitVerifyMaxResends]time.Duration) {
	t.Helper()
	saved := submitVerifyBackoff
	submitVerifyBackoff = steps
	t.Cleanup(func() { submitVerifyBackoff = saved })
}

func realClockDeps(tm TmuxController, stderr *bytes.Buffer) Deps {
	return Deps{Tmux: tm, Stderr: stderr, LookupEnv: mapLookup(nil)}.withDefaults()
}

type verifyRun struct {
	outcome submitVerifyOutcome
	elapsed time.Duration
	timeout bool
}

func runVerifyUntil(ctx context.Context, tm TmuxController, limit time.Duration) verifyRun {
	var stderr bytes.Buffer
	lp := tmuxLaunch{name: "claude-tmux", session: "s",
		promptMarker: tmuxPromptMarkerDefault, inputLineMarker: tmuxPromptMarkerDefault}
	done := make(chan verifyRun, 1)
	start := time.Now()
	go func() {
		o := verifySubmitted(ctx, realClockDeps(tm, &stderr), lp, "[claude-tmux]", "nudge",
			parkedPane(guardNudge), guardNudge)
		done <- verifyRun{outcome: o, elapsed: time.Since(start)}
	}()
	select {
	case r := <-done:
		return r
	case <-time.After(limit):
		return verifyRun{timeout: true, elapsed: limit}
	}
}

func TestVerifySubmitted_CancelDuringBackoffEndsTheWaitWithinOnePoll(t *testing.T) {
	withBackoff(t, [submitVerifyMaxResends]time.Duration{
		time.Millisecond, time.Millisecond, time.Millisecond, 10 * time.Second, time.Millisecond, time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tm := &cancelOnSendTmux{
		fakeTmux: &fakeTmux{paneSeq: []string{parkedPane(guardNudge)}},
		cancel:   cancel, cancelAt: 4, cancelAfter: 50 * time.Millisecond,
	}

	got := runVerifyUntil(ctx, tm, artifactWaitInterval)

	if got.timeout {
		t.Fatalf("verifySubmitted still waiting %v after the context was cancelled mid-backoff "+
			"(poll = %v): the re-send wait ignores ctx", got.elapsed, artifactWaitInterval)
	}
	if got.outcome.Result != interaction.ResultNotVerified {
		t.Errorf("outcome = %+v, want Result:%s — a cancelled wait cannot claim the submission state",
			got.outcome, interaction.ResultNotVerified)
	}
}

func TestVerifySubmitted_CancelDuringFourthBackoffStopsFurtherSendKeys(t *testing.T) {
	withBackoff(t, [submitVerifyMaxResends]time.Duration{
		time.Millisecond, time.Millisecond, time.Millisecond, 200 * time.Millisecond, time.Millisecond, time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tm := &cancelOnSendTmux{
		fakeTmux: &fakeTmux{paneSeq: []string{parkedPane(guardNudge)}},
		cancel:   cancel, cancelAt: 4, cancelAfter: 20 * time.Millisecond,
	}

	got := runVerifyUntil(ctx, tm, 5*time.Second)

	if got.timeout {
		t.Fatalf("verifySubmitted did not return within %v", got.elapsed)
	}
	if n := int(tm.sends.Load()); n != 4 {
		t.Errorf("SendKeys ran %d time(s), want exactly 4 — a re-send went out after the context was cancelled", n)
	}
	if got.outcome.Result != interaction.ResultNotVerified {
		t.Errorf("outcome = %+v, want Result:%s", got.outcome, interaction.ResultNotVerified)
	}
}

func TestVerifySubmitted_CancelAlreadyPendingWhenTheWaitStartsSendsNothingMore(t *testing.T) {
	withBackoff(t, [submitVerifyMaxResends]time.Duration{
		time.Millisecond, time.Millisecond, time.Millisecond, time.Millisecond, time.Millisecond, time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tm := &cancelOnSendTmux{
		fakeTmux: &fakeTmux{paneSeq: []string{parkedPane(guardNudge)}},
		cancel:   cancel, cancelAt: 4,
	}

	got := runVerifyUntil(ctx, tm, 5*time.Second)

	if got.timeout {
		t.Fatalf("verifySubmitted did not return within %v", got.elapsed)
	}
	if n := int(tm.sends.Load()); n != 4 {
		t.Errorf("SendKeys ran %d time(s), want exactly 4 — the loop kept re-sending on a dead context", n)
	}
	if got.outcome.Result != interaction.ResultNotVerified {
		t.Errorf("outcome = %+v, want Result:%s", got.outcome, interaction.ResultNotVerified)
	}
}

type parkedForResendsTmux struct {
	*fakeTmux
	mu      sync.Mutex
	trigger string
	parked  bool
	text    string
	left    int
}

func (p *parkedForResendsTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	p.mu.Lock()
	switch {
	case !p.parked && p.left > 0 && strings.Contains(keys, p.trigger):
		p.parked, p.text = true, keys
	case p.parked && keys == "" && enter:
		p.left--
		if p.left <= 0 {
			p.parked = false
		}
	}
	p.mu.Unlock()
	return p.fakeTmux.SendKeys(ctx, session, keys, enter)
}

func (p *parkedForResendsTmux) CapturePane(ctx context.Context, session string, scrollback int) (string, error) {
	p.mu.Lock()
	parked, text := p.parked, p.text
	p.mu.Unlock()
	out, err := p.fakeTmux.CapturePane(ctx, session, scrollback)
	if parked {
		return unsubmittedPane(text), err
	}
	return out, err
}

func pollsUntilTimeout(t *testing.T, resends int) int {
	t.Helper()
	fx := newFixture(t, "claude-tmux", "")
	tm := &parkedForResendsTmux{
		fakeTmux: &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}},
		trigger:  fx.artifact,
		left:     resends,
	}
	var polls atomic.Int32
	eng := NewEngine(Deps{
		Tmux: tm,
		Sleep: func(d time.Duration) {
			if d == artifactWaitInterval {
				polls.Add(1)
			}
		},
		LookupEnv: mapLookup(nil),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(ctx, fx.args("claude-tmux", "--allow-bypass", "--agent=build", "--cycle=1526"), nil, &stdout, &stderr)
	if code != ExitArtifactTimeout {
		t.Fatalf("resends=%d: exit = %d, want ExitArtifactTimeout", resends, code)
	}
	return int(polls.Load())
}

func TestNudgeSite_ResendWindowCountsInElapsedWaitTime(t *testing.T) {
	t.Parallel()
	const resends = 4
	var window time.Duration
	for _, d := range submitVerifyBackoff[:resends] {
		window += d
	}

	baseline := pollsUntilTimeout(t, 0)
	withResends := pollsUntilTimeout(t, resends)

	floor := int(window/artifactWaitInterval) - 1
	if got := baseline - withResends; got < floor {
		t.Errorf("polls to timeout: %d with %d re-sends vs %d with none (saved %d), want saved >= %d — "+
			"the %v re-send window is wall time the wait loop never counted toward elapsed",
			withResends, resends, baseline, got, floor, window)
	}
}

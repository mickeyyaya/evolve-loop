package usageprobe

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

func recordWindows(t *testing.T, dir, cli string, wins ...quotastate.UsageWindow) {
	t.Helper()
	if err := RecordObservation(dir, Observation{CLI: cli, ObservedAt: time.Now(), Windows: wins}); err != nil {
		t.Fatal(err)
	}
}

func TestAmplifiedHold_NegativeLimitKeepsTheBrakeOff(t *testing.T) {
	dir := t.TempDir()
	recordWindows(t, dir, "claude", weekWindow(100, "", nil))
	if h := CheckHold(dir, -5, time.Now()); h.Held || h.Unreadable {
		t.Errorf("hold = %+v, want off for a negative limit", h)
	}
}

func TestAmplifiedHold_HundredPercentLimitHoldsAtHundred(t *testing.T) {
	dir := t.TempDir()
	recordWindows(t, dir, "claude", weekWindow(100, "", nil))
	if h := CheckHold(dir, 100, time.Now()); !h.Held || h.Percent != 100 {
		t.Errorf("hold = %+v, want held at 100", h)
	}
}

func TestAmplifiedHold_FractionalLimitBoundary(t *testing.T) {
	dir := t.TempDir()
	recordWindows(t, dir, "claude", weekWindow(87.5, "", nil))
	if h := CheckHold(dir, 87.5, time.Now()); !h.Held {
		t.Errorf("hold = %+v, want held at the exact fractional limit", h)
	}
	if h := CheckHold(dir, 87.6, time.Now()); h.Held {
		t.Errorf("hold = %+v, want no hold just above the reading", h)
	}
}

func TestAmplifiedHold_TheHighestLiveFamilyDecides(t *testing.T) {
	dir := t.TempDir()
	recordWindows(t, dir, "claude", weekWindow(40, "", nil))
	codex := weekWindow(95, "", nil)
	codex.Family = "codex"
	recordWindows(t, dir, "codex", codex)
	if h := CheckHold(dir, 90, time.Now()); !h.Held || h.Percent != 95 {
		t.Errorf("hold = %+v, want held at the highest reading 95", h)
	}
}

func TestAmplifiedHold_ANonWeekWindowIsNotTheWeekReading(t *testing.T) {
	dir := t.TempDir()
	w := weekWindow(99, "", nil)
	w.Kind = quotastate.KindSession
	recordWindows(t, dir, "claude", w, weekWindow(10, "", nil))
	if h := CheckHold(dir, 90, time.Now()); h.Held {
		t.Errorf("hold = %+v, want the session window ignored", h)
	}
}

func TestAmplifiedHold_AWindowResettingExactlyNowIsNotLive(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2099, 5, 5, 5, 5, 5, 0, time.UTC)
	past := now.Add(-time.Second)
	recordWindows(t, dir, "claude", weekWindow(99, "", &past))
	if h := CheckHold(dir, 90, now); h.Held {
		t.Errorf("hold = %+v, want no hold for a window reset a second ago", h)
	}
}

func TestAmplifiedHold_BrakeOffIgnoresACorruptFile(t *testing.T) {
	dir := t.TempDir()
	recordWindows(t, dir, "claude", weekWindow(99, "", nil))
	if h := CheckHold(dir, 0, time.Now()); h.Held || h.Unreadable {
		t.Errorf("hold = %+v, want off", h)
	}
}

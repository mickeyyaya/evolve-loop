package usageprobe

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

func TestCheckHold_HoldsAtTheLimitAndNamesTheReadingAndTheReset(t *testing.T) {
	dir := t.TempDir()
	resets := time.Date(2099, 11, 20, 4, 0, 0, 0, time.UTC)
	obs := Observation{CLI: "claude", ObservedAt: time.Now(), Windows: []quotastate.UsageWindow{{
		Kind: quotastate.KindWeek, PercentUsed: 90, Family: "claude", ResetsText: "Nov 20", ResetsAt: &resets,
	}}}
	if err := RecordObservation(dir, obs); err != nil {
		t.Fatal(err)
	}
	h := CheckHold(dir, 90, time.Now())
	if !h.Held || h.Unreadable || h.Percent != 90 {
		t.Fatalf("hold = %+v, want held at 90", h)
	}
	for _, want := range []string{"90", "Nov 20", "2099-11-20T04:00:00Z"} {
		if !strings.Contains(h.Message(dir), want) {
			t.Errorf("message %q lacks %q", h.Message(dir), want)
		}
	}
	if CheckHold(dir, 0, time.Now()).Held {
		t.Error("a zero limit must keep the brake off")
	}
	if CheckHold(dir, 91, time.Now()).Held {
		t.Error("a reading under the limit must not hold")
	}
}

func weekWindow(pct float64, text string, at *time.Time) quotastate.UsageWindow {
	return quotastate.UsageWindow{Kind: quotastate.KindWeek, PercentUsed: pct, Family: "claude", ResetsText: text, ResetsAt: at}
}

func TestCheckHold_ReadingsAndTheResetText(t *testing.T) {
	at := time.Date(2099, 1, 2, 3, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		win  quotastate.UsageWindow
		want string
	}{
		{"instant only", weekWindow(95, "", &at), "2099-01-02T03:00:00Z"},
		{"text only", weekWindow(95, "Jan 2", nil), "Jan 2"},
		{"neither", weekWindow(95, "", nil), "at an unknown time"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := RecordObservation(dir, Observation{CLI: "claude", ObservedAt: time.Now(), Windows: []quotastate.UsageWindow{tc.win}}); err != nil {
				t.Fatal(err)
			}
			var h Hold = CheckHold(dir, 90, time.Now())
			if !h.Held || !strings.Contains(h.Message(dir), tc.want) {
				t.Errorf("hold %+v message %q, want held and %q", h, h.Message(dir), tc.want)
			}
		})
	}
}

func TestCheckHold_AnUnreadableReadingHoldsAndNamesTheFile(t *testing.T) {
	corrupt := t.TempDir()
	if err := os.WriteFile(WindowsPath(corrupt), []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	failed := t.TempDir()
	if err := RecordObservation(failed, Observation{CLI: "claude", ObservedAt: time.Now(), Error: "no pane"}); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{corrupt, failed} {
		h := CheckHold(dir, 90, time.Now())
		msg := h.Message(dir)
		if !h.Held || !h.Unreadable || !strings.Contains(msg, "unreadable") || !strings.Contains(msg, WindowsFile) {
			t.Errorf("hold %+v message %q, want an unreadable hold naming %s", h, msg, WindowsFile)
		}
	}
}

func TestCheckHold_AFailedProbeBesideALiveReadingKeepsTheReading(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2099, 1, 2, 3, 0, 0, 0, time.UTC)
	if err := RecordObservation(dir, Observation{CLI: "codex", ObservedAt: time.Now(), Error: "no pane"}); err != nil {
		t.Fatal(err)
	}
	if err := RecordObservation(dir, Observation{CLI: "claude", ObservedAt: time.Now(), Windows: []quotastate.UsageWindow{weekWindow(50, "", &at)}}); err != nil {
		t.Fatal(err)
	}
	if h := CheckHold(dir, 90, time.Now()); h.Held || h.Unreadable || h.Percent != 50 {
		t.Errorf("hold = %+v, want a live 50%% reading under the limit", h)
	}
}

func TestCheckHold_AResetWindowIsNotALiveReading(t *testing.T) {
	dir := t.TempDir()
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := RecordObservation(dir, Observation{CLI: "claude", ObservedAt: time.Now(), Windows: []quotastate.UsageWindow{weekWindow(99, "", &past)}}); err != nil {
		t.Fatal(err)
	}
	if h := CheckHold(dir, 90, time.Now()); h.Held || h.Unreadable {
		t.Errorf("hold = %+v, want no hold after the window reset", h)
	}
}

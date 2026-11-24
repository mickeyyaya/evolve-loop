package seatlogin

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

func TestInspect_NamesEachStateOfTheHolderAndTheSeatDirectory(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		build func(t *testing.T) string
		want  State
		text  string
	}{
		{"no holder and no seat directory", func(t *testing.T) string { return t.TempDir() }, StateNone, ""},
		{"a valid holder", func(t *testing.T) string { return plant(t, 0o600, canary) }, StateIsolated, ""},
		{"an empty holder", func(t *testing.T) string { return plant(t, 0o600, "") }, StateBad, "is empty"},
		{"a holder with a wrong mode", func(t *testing.T) string { return plant(t, 0o644, canary) }, StateBad, "not 0600"},
		{"a seat directory with no holder", func(t *testing.T) string {
			home := t.TempDir()
			if err := os.MkdirAll(paths.SeatRoot(home), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(paths.SeatSettingsPath(home), []byte("{}"), 0o600); err != nil {
				t.Fatal(err)
			}
			return home
		}, StateOrphan, "holder is absent"},
		{"a seat directory an unarmed wrap created", func(t *testing.T) string {
			home := t.TempDir()
			if err := os.MkdirAll(paths.SeatHolderDir(home), 0o700); err != nil {
				t.Fatal(err)
			}
			return home
		}, StateNone, ""},
		{"a seat root that is a file", func(t *testing.T) string {
			home := t.TempDir()
			if err := os.WriteFile(paths.SeatRoot(home), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			return home
		}, StateFault, "not a directory"},
	}
	for _, c := range cases {
		got := Inspect(c.build(t), OS())
		if got.State != c.want {
			t.Errorf("%s: state %v, want %v (%q)", c.name, got.State, c.want, got.Message)
		}
		if !strings.Contains(got.Message, c.text) {
			t.Errorf("%s: message %q lacks %q", c.name, got.Message, c.text)
		}
		if strings.Contains(got.Message, canary) {
			t.Errorf("%s: the message holds the token", c.name)
		}
	}
}

func TestInspect_AnEmptyHomeAndAnUnreadableSeatRootAreFaults(t *testing.T) {
	t.Parallel()
	if got := Inspect("", OS()); got.State != StateFault {
		t.Errorf("empty home: %v", got.State)
	}
	deps := Deps{
		Stat: func(p string) (fs.FileInfo, error) {
			if strings.HasSuffix(p, ".evolve-seat") {
				return nil, errors.New("denied")
			}
			return nil, fs.ErrNotExist
		},
		ReadFile: os.ReadFile,
	}
	if got := Inspect("/h", deps); got.State != StateFault || !strings.Contains(got.Message, "denied") {
		t.Errorf("got %+v", got)
	}
}

func TestHealth_ABadHolderCarriesTheBadStateAndTheRefusalReason(t *testing.T) {
	t.Parallel()
	home := plant(t, 0o644, canary)
	var h Health = Inspect(home, OS())
	if h.State != StateBad {
		t.Fatalf("State %v, want %v", h.State, StateBad)
	}
	if !strings.Contains(h.Message, "has mode 0644, not 0600") || strings.Contains(h.Message, canary) {
		t.Fatalf("Message %q", h.Message)
	}
	good := Inspect(plant(t, 0o600, canary), OS())
	if good.State != StateIsolated || good == h {
		t.Fatalf("a good holder read as %+v", good)
	}
}

func holderAged(t *testing.T, age time.Duration, now time.Time) string {
	t.Helper()
	home := plant(t, 0o600, canary)
	mtime := now.Add(-age)
	if err := os.Chtimes(paths.SeatTokenPath(home), mtime, mtime); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestInspect_AHolderAtTheWarnAgeIsExpiringAndOneDayYoungerIsIsolated(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	deps := OS()
	deps.Now = func() time.Time { return now }
	day := 24 * time.Hour
	cases := []struct {
		name string
		age  time.Duration
		want State
		text string
	}{
		{"one day under the warn age", time.Duration(TokenWarnAgeDays-1)*day + 23*time.Hour, StateIsolated, "329 days old"},
		{"the warn age", time.Duration(TokenWarnAgeDays) * day, StateExpiring, "330 days old"},
		{"over the warn age", time.Duration(TokenWarnAgeDays+1) * day, StateExpiring, "331 days old"},
		{"a new holder", 0, StateIsolated, "0 days old"},
		{"a holder dated in the future", -3 * day, StateIsolated, "0 days old"},
	}
	for _, c := range cases {
		got := Inspect(holderAged(t, c.age, now), deps)
		if got.State != c.want || !strings.Contains(got.Message, c.text) {
			t.Errorf("%s: %+v, want state %v with %q", c.name, got, c.want, c.text)
		}
		if strings.Contains(got.Message, canary) {
			t.Errorf("%s: the message holds the token", c.name)
		}
	}
}

func TestInspect_AnExpiringMessageNamesTheRenewalSteps(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	deps := OS()
	deps.Now = func() time.Time { return now }
	got := Inspect(holderAged(t, 340*24*time.Hour, now), deps)
	for _, want := range []string{"claude setup-token", "evolve auth seat-token set"} {
		if !strings.Contains(got.Message, want) {
			t.Errorf("message %q lacks %q", got.Message, want)
		}
	}
}

func TestInspect_ANilClockUsesTheWallClock(t *testing.T) {
	t.Parallel()
	deps := Deps{Stat: lstat, ReadFile: os.ReadFile}
	if got := Inspect(plant(t, 0o600, canary), deps); got.State != StateIsolated || !strings.Contains(got.Message, "0 days old") {
		t.Fatalf("got %+v", got)
	}
}

func TestInspect_TheHolderIsStatedOnceSoTheAgeAndTheTokenAgree(t *testing.T) {
	t.Parallel()
	home := plant(t, 0o600, canary)
	holder := paths.SeatTokenPath(home)
	calls := 0
	deps := OS()
	deps.Stat = func(p string) (fs.FileInfo, error) {
		if p == holder {
			calls++
		}
		return os.Lstat(p)
	}
	Inspect(home, deps)
	if calls != 1 {
		t.Fatalf("the holder was stated %d times, want 1", calls)
	}
}

func TestInspect_ANoneMessageNamesTheNextStep(t *testing.T) {
	t.Parallel()
	got := Inspect(t.TempDir(), OS())
	for _, want := range []string{"claude setup-token", "evolve auth seat-token set"} {
		if !strings.Contains(got.Message, want) {
			t.Errorf("message %q lacks %q", got.Message, want)
		}
	}
}

func TestState_SeverityNamesEachStateAndAnUnlistedStateIsUnknown(t *testing.T) {
	t.Parallel()
	want := map[State]Severity{
		StateNone:     SeverityOK,
		StateIsolated: SeverityOK,
		StateBad:      SeverityHalt,
		StateOrphan:   SeverityWarn,
		StateFault:    SeverityWarn,
		StateExpiring: SeverityWarn,
	}
	for state, severity := range want {
		if got := state.Severity(); got != severity {
			t.Errorf("state %v: %v, want %v", state, got, severity)
		}
	}
	if got := State(99).Severity(); got != SeverityUnknown {
		t.Errorf("an unlisted state: %v, want unknown", got)
	}
	if SeverityUnknown != 0 {
		t.Error("the zero severity must be unknown, so a state with no entry never reads as ok")
	}
}

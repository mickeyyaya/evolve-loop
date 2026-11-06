package seatlogin

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

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

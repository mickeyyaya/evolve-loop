package paths

import (
	"path/filepath"
	"testing"
)

func TestSeatPaths(t *testing.T) {
	t.Parallel()
	home := filepath.Join("h", "u")
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"root", SeatRoot(home), filepath.Join(home, ".evolve-seat")},
		{"holder", SeatHolderDir(home), filepath.Join(home, ".evolve-seat", "holder")},
		{"settings", SeatSettingsPath(home), filepath.Join(home, ".evolve-seat", "settings.json")},
		{"token", SeatTokenPath(home), filepath.Join(home, ".evolve-seat", "holder", "token")},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestSeatPaths_AnEmptyHomeIsTheCallersRefusal(t *testing.T) {
	t.Parallel()
	for name, got := range map[string]string{"root": SeatRoot(""), "holder": SeatHolderDir(""), "token": SeatTokenPath(""), "settings": SeatSettingsPath("")} {
		if got != "" {
			t.Errorf("%s of an empty home = %q, want the empty string so that no path rooted at / exists", name, got)
		}
	}
}

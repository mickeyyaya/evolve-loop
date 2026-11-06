package opscmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/seatlogin"
)

func TestDoctorSeatInspect_ReadsTheHomeOfTheProcess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := paths.SeatTokenPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(" "), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := doctorSeatInspect(); got.State != seatlogin.StateBad {
		t.Fatalf("got %+v: the check did not read the holder of the home", got)
	}
}

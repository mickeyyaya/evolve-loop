package bridge

import (
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/seatlogin"
)

var realHomeAtStart, _ = os.UserHomeDir()

func init() {
	seatLoginFallback = func() seatlogin.Deps { return hidingHome(realHomeAtStart, seatlogin.OS()) }
}

func TestSeatLoginSeam_AnEmptySeamNeverSeesTheHolderOfTheRealHome(t *testing.T) {
	t.Parallel()
	deps := Deps{}.seatLoginDeps()
	for _, p := range []string{paths.SeatTokenPath(realHomeAtStart), paths.SeatRoot(realHomeAtStart)} {
		if _, err := deps.Stat(p); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Stat(%s) err=%v: a bridge test must not see the real holder", p, err)
		}
	}
}

func TestSeatLoginSeam_AnEmptySeamStillReadsATempHome(t *testing.T) {
	home := seatHome(t)
	plantHolder(t, home, seatCanaryToken, 0o600)
	d := seatlogin.Resolve(home, "", Deps{}.seatLoginDeps())
	if !d.HostIsolated || d.SeatToken != seatCanaryToken {
		t.Fatalf("got %+v", d)
	}
}

func TestSeatLoginSeam_AnExplicitSeamWins(t *testing.T) {
	t.Parallel()
	want := seatlogin.Deps{Stat: func(string) (fs.FileInfo, error) { return nil, errors.New("explicit") }, ReadFile: os.ReadFile}
	if _, err := (Deps{SeatLogin: want}).seatLoginDeps().Stat("x"); err == nil || err.Error() != "explicit" {
		t.Fatalf("err=%v", err)
	}
}

package bridge

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/seatlogin"
)

func hidingHome(home string, inner seatlogin.Deps) seatlogin.Deps {
	root := paths.SeatRoot(home)
	hidden := func(path string) bool {
		return root != "" && (path == root || strings.HasPrefix(path, root+string(filepath.Separator)))
	}
	return seatlogin.Deps{
		Stat: func(path string) (fs.FileInfo, error) {
			if hidden(path) {
				return nil, fs.ErrNotExist
			}
			return inner.Stat(path)
		},
		ReadFile: func(path string) ([]byte, error) {
			if hidden(path) {
				return nil, fs.ErrNotExist
			}
			return inner.ReadFile(path)
		},
	}
}

func plantAt(t *testing.T, home string) string {
	t.Helper()
	path := paths.SeatTokenPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(seatCanaryToken), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestHidingHome_TheSeatRootOfTheNamedHomeIsAbsentAndAnotherHomeIsRead(t *testing.T) {
	t.Parallel()
	real, other := t.TempDir(), t.TempDir()
	hidden := plantAt(t, real)
	plantAt(t, other)
	deps := hidingHome(real, seatlogin.OS())
	for _, p := range []string{paths.SeatRoot(real), paths.SeatHolderDir(real), paths.SeatTokenPath(real)} {
		if _, err := deps.Stat(p); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Stat(%s) err=%v, want not-exist", p, err)
		}
		if _, err := deps.ReadFile(p); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("ReadFile(%s) err=%v, want not-exist", p, err)
		}
	}
	if _, err := os.Stat(hidden); err != nil {
		t.Fatalf("the fixture is not planted: %v", err)
	}
	if got := seatlogin.Resolve(other, "", deps); !got.HostIsolated || got.SeatToken != seatCanaryToken {
		t.Errorf("another home must resolve as today: %+v", got)
	}
	if got := seatlogin.Resolve(real, "", deps); got != (seatlogin.Decision{}) {
		t.Errorf("the hidden home must resolve as no holder: %+v", got)
	}
}

func TestHidingHome_AnEmptyHomeHidesNothing(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	plantAt(t, home)
	if got := seatlogin.Resolve(home, "", hidingHome("", seatlogin.OS())); !got.HostIsolated {
		t.Fatalf("got %+v", got)
	}
}

package tmuxtest

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

func TestStopFamily_RemovesTheServerAndEverySuffixedServerOfTheSocket(t *testing.T) {
	socketDir := privateSocketDir(t)
	socket := bridge.DeriveTestSocket(os.Getpid())
	for _, name := range []string{socket, socket + "-own"} {
		if err := StartServer(name); err != nil {
			t.Fatal(err)
		}
	}

	err := stopFamily(socket)

	if err != nil {
		t.Fatalf("stopFamily = %v, want nil", err)
	}
	left, _ := filepath.Glob(filepath.Join(socketDir, socket+"*"))
	if len(left) != 0 {
		t.Errorf("stopFamily left %v", left)
	}
}

func TestStopFamily_ReportsABadSocketPattern(t *testing.T) {
	privateSocketDir(t)

	err := stopFamily("evolve-bridge-t[")

	if !errors.Is(err, filepath.ErrBadPattern) {
		t.Errorf("stopFamily = %v, want filepath.ErrBadPattern", err)
	}
}

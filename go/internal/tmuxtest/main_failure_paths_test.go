package tmuxtest

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

func TestMain_ASocketThatCannotBeSelectedRunsNothingAndFails(t *testing.T) {
	prev := setenv
	t.Cleanup(func() { setenv = prev })
	setenv = func(string, string) error { return errors.New("env frozen") }
	ran := false

	code := Main(runFunc(func() int { ran = true; return 0 }))

	if code != 1 || ran {
		t.Fatalf("Main = %d, ran = %v; want 1 without running the tests", code, ran)
	}
}

func TestMain_AServerThatCannotBeStoppedFailsEvenAGreenRun(t *testing.T) {
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "tmux"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin)
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	t.Setenv(bridge.TmuxSocketEnv, bridge.DeriveRunSocket(1))
	cases := []struct {
		name    string
		runCode int
		want    int
	}{
		{"green run becomes red", 0, 1},
		{"red run keeps its code", 4, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stuck := filepath.Join(bridge.TmuxSocketDir(), bridge.DeriveTestSocket(os.Getpid()))
			t.Cleanup(func() { _ = os.RemoveAll(stuck) })

			code := Main(runFunc(func() int {
				if err := os.MkdirAll(filepath.Join(stuck, "occupied"), 0o755); err != nil {
					t.Fatal(err)
				}
				return tc.runCode
			}))

			if code != tc.want {
				t.Errorf("Main = %d, want %d when the socket cannot be removed after the run", code, tc.want)
			}
		})
	}
}

//go:build notmux

package tmuxtest

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

func floorTmuxHome(t *testing.T) (calls string, socketDir string) {
	t.Helper()
	bin := t.TempDir()
	calls = filepath.Join(t.TempDir(), "calls")
	fakeclitest.Install(t, filepath.Join(bin, "tmux"), "#!/bin/sh\necho \"$*\" >> "+calls+"\nexit 0\n")
	t.Setenv("PATH", bin)
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	t.Setenv(bridge.TmuxSocketEnv, bridge.DeriveRunSocket(1))
	return calls, bridge.TmuxSocketDir()
}

func TestMain_AtTheFloorRunsTheTestsOnThePrivateSocketAndStartsNoServer(t *testing.T) {
	calls, _ := floorTmuxHome(t)
	var socket string

	code := Main(runFunc(func() int { socket = os.Getenv(bridge.TmuxSocketEnv); return 3 }))

	if code != 3 || socket != bridge.DeriveTestSocket(os.Getpid()) {
		t.Errorf("Main = %d on socket %q, want 3 on %q", code, socket, bridge.DeriveTestSocket(os.Getpid()))
	}
	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		t.Errorf("tmux was called at the floor (stat err %v)", err)
	}
}

func TestMain_AtTheFloorFailsAGreenRunThatLeftAServerAndStopsIt(t *testing.T) {
	calls, _ := floorTmuxHome(t)
	own := bridge.DeriveTestSocket(os.Getpid())
	prev := strayCheck
	t.Cleanup(func() { strayCheck = prev })
	strayCheck = func(string) ([]string, error) { return []string{own + "-own"}, nil }

	code := Main(runFunc(func() int { return 0 }))

	if code != 1 {
		t.Errorf("Main = %d, want 1 after a stray server", code)
	}
	log, _ := os.ReadFile(calls)
	if !strings.Contains(string(log), "kill-server") {
		t.Errorf("tmux calls:\n%s\nwant a kill-server of the own server", log)
	}
}

func TestMain_AtTheFloorKeepsTheCodeOfARedRunAndNamesTheReach(t *testing.T) {
	floorTmuxHome(t)
	prev := strayCheck
	t.Cleanup(func() { strayCheck = prev })
	strayCheck = func(string) ([]string, error) { return []string{"s"}, nil }
	stderr := captureStderr(t, func() {
		if code := Main(runFunc(func() int { return 4 })); code != 4 {
			t.Errorf("Main = %d, want 4", code)
		}
	})

	if !strings.Contains(stderr, "tmux reached at the floor") || !strings.Contains(stderr, " s") {
		t.Errorf("stderr = %q, want the reach named", stderr)
	}
}

func TestMain_AtTheFloorReportsAServerThatCannotBeStopped(t *testing.T) {
	_, socketDir := floorTmuxHome(t)
	stuck := filepath.Join(socketDir, bridge.DeriveTestSocket(os.Getpid()))
	if err := os.MkdirAll(filepath.Join(stuck, "occupied"), 0o755); err != nil {
		t.Fatal(err)
	}
	prev := strayCheck
	t.Cleanup(func() { strayCheck = prev })
	strayCheck = func(string) ([]string, error) { return []string{"s"}, nil }
	stderr := captureStderr(t, func() {
		if code := Main(runFunc(func() int { return 0 })); code != 1 {
			t.Errorf("Main = %d, want 1", code)
		}
	})

	if !strings.Contains(stderr, "directory not empty") {
		t.Errorf("stderr = %q, want the stop failure", stderr)
	}
}

func TestMain_AtTheFloorFailsWhenTheStrayCheckCannotRun(t *testing.T) {
	floorTmuxHome(t)
	prev := strayCheck
	t.Cleanup(func() { strayCheck = prev })
	strayCheck = func(string) ([]string, error) { return nil, errors.New("unreadable") }
	stderr := captureStderr(t, func() {
		if code := Main(runFunc(func() int { return 0 })); code != 1 {
			t.Errorf("Main = %d, want 1", code)
		}
	})

	if !strings.Contains(stderr, "unreadable") {
		t.Errorf("stderr = %q, want the check failure", stderr)
	}
}

func TestStrayServers_FindsTheOwnSocketAndItsSuffixedFamilyOnly(t *testing.T) {
	_, socketDir := floorTmuxHome(t)
	if got, err := strayServers("evolve-t1"); err != nil || got != nil {
		t.Fatalf("no socket directory: %v, %v, want nothing", got, err)
	}
	if err := os.MkdirAll(socketDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"evolve-t1", "evolve-t1-own", "evolve-t12", "other"} {
		if err := os.WriteFile(filepath.Join(socketDir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := strayServers("evolve-t1")

	if err != nil || strings.Join(got, ",") != "evolve-t1,evolve-t1-own" {
		t.Errorf("strayServers = %v, %v, want the socket and its own server", got, err)
	}
}

func TestStrayServers_AnUnreadableSocketDirectoryIsAnError(t *testing.T) {
	_, socketDir := floorTmuxHome(t)
	if err := os.WriteFile(socketDir, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	if got, err := strayServers("evolve-t1"); err == nil || got != nil {
		t.Errorf("strayServers = %v, %v, want an error", got, err)
	}
}

func captureStderr(t *testing.T, run func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	read := make(chan []byte)
	go func() {
		data, _ := io.ReadAll(r)
		read <- data
	}()
	prev := os.Stderr
	os.Stderr = w
	run()
	os.Stderr = prev
	_ = w.Close()
	data := <-read
	_ = r.Close()
	return string(data)
}

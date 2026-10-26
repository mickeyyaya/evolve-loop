package tmuxtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

func serverAlive(socket string) bool {
	return exec.Command("tmux", "-L", socket, "has-session", "-t", bridge.ExactSessionTarget("victim")).Run() == nil
}

func TestStartOrphanGuard_TheServersOfTheOwnerStopSoonAfterItDies(t *testing.T) {
	privateSocketDir(t)
	socket := bridge.DeriveTestSocket(os.Getpid())
	t.Setenv(bridge.TmuxSocketEnv, socket)
	owner := exec.Command("sleep", "60")
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Process.Kill(); _, _ = owner.Process.Wait() })
	if err := StartServer(socket); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = StopServer(socket) })
	if err := startOrphanGuard(socket, owner.Process.Pid); err != nil {
		t.Fatal(err)
	}
	own, lookalike := socket+"-own", socket+"9"
	for _, s := range []string{socket, own, lookalike} {
		if out, err := exec.Command("tmux", "-L", s, "-f", os.DevNull, "new-session", "-d", "-s", "victim", "sleep 600").CombinedOutput(); err != nil {
			t.Fatalf("new-session on %s: %v: %s", s, err, out)
		}
	}
	t.Cleanup(func() { _ = StopServer(own); _ = StopServer(lookalike) })

	time.Sleep(1500 * time.Millisecond)
	if !serverAlive(socket) || !serverAlive(own) {
		t.Fatal("the guard stopped a server while its owner still runs")
	}
	if err := owner.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_, _ = owner.Process.Wait()

	deadline := time.Now().Add(10 * time.Second)
	for serverAlive(socket) || serverAlive(own) {
		if time.Now().After(deadline) {
			t.Fatalf("a server of the owner outlived it by 10 s (main %v, own %v); a timeout panic leaves it to hold the CI step open", serverAlive(socket), serverAlive(own))
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !serverAlive(lookalike) {
		t.Fatalf("the guard stopped %s, the server of another pid whose name only starts like %s", lookalike, socket)
	}
}

func TestStartOrphanGuard_AGuardThatCannotStartIsAnError(t *testing.T) {
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "tmux"), "#!/bin/sh\necho refused >&2\nexit 1\n")
	t.Setenv("PATH", bin)

	err := startOrphanGuard(bridge.DeriveTestSocket(os.Getpid()), os.Getpid())

	if err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("startOrphanGuard err = %v, want the tmux refusal", err)
	}
}

func TestMain_AGuardThatCannotStartStopsTheServerAndRunsNothing(t *testing.T) {
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "calls")
	fakeclitest.Install(t, filepath.Join(bin, "tmux"), "#!/bin/sh\necho \"$*\" >> "+log+"\ncase \"$*\" in *new-session*) exit 1 ;; esac\nexit 0\n")
	t.Setenv("PATH", bin)
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	ran := false

	code := Main(runFunc(func() int { ran = true; return 0 }))

	calls, _ := os.ReadFile(log)
	if code != 1 || ran || !strings.HasSuffix(strings.TrimSpace(string(calls)), "kill-server") {
		t.Fatalf("Main = %d, ran = %v, tmux calls:\n%s\nwant 1, no run, and a stopped server", code, ran, calls)
	}
}

func readEOF(r *os.File) chan struct{} {
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 1)
		for {
			if _, err := r.Read(buf); err != nil {
				close(done)
				return
			}
		}
	}()
	return done
}

func TestCloseInheritedOnExec_AChildThatOutlivesTheTestHoldsNoInheritedPipe(t *testing.T) {
	var fds [2]int
	if err := syscall.Pipe(fds[:]); err != nil {
		t.Fatal(err)
	}
	r, w := os.NewFile(uintptr(fds[0]), "r"), os.NewFile(uintptr(fds[1]), "w")
	t.Cleanup(func() { _ = r.Close() })

	if err := closeInheritedOnExec(); err != nil {
		t.Fatal(err)
	}
	child := exec.Command("sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = child.Process.Kill(); _, _ = child.Process.Wait() })
	_ = w.Close()

	select {
	case <-readEOF(r):
	case <-time.After(5 * time.Second):
		t.Fatal("a child still holds the inherited pipe, so a CI step that reads it waits for the child (the #1132 hang)")
	}
}

func TestCloseInheritedOnExec_AnUnreadableDescriptorListIsAnError(t *testing.T) {
	prev := fdDir
	t.Cleanup(func() { fdDir = prev })
	cases := map[string]func(t *testing.T) string{
		"missing directory": func(t *testing.T) string { return filepath.Join(t.TempDir(), "none") },
		"a name that is not a descriptor": func(t *testing.T) string {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "x"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			return dir
		},
	}
	for name, dir := range cases {
		fdDir = dir(t)
		if err := closeInheritedOnExec(); err == nil {
			t.Errorf("%s: closeInheritedOnExec must fail loudly, got nil", name)
		}
	}
}

func TestMain_AnUnreadableDescriptorListRunsNothingAndFails(t *testing.T) {
	prev := fdDir
	t.Cleanup(func() { fdDir = prev })
	fdDir = filepath.Join(t.TempDir(), "none")
	t.Setenv(bridge.TmuxSocketEnv, bridge.DeriveRunSocket(1))
	ran := false

	code := Main(runFunc(func() int { ran = true; return 0 }))

	if code != 1 || ran {
		t.Fatalf("Main = %d, ran = %v; want 1 without running the tests", code, ran)
	}
}

func TestShellQuote_KeepsAQuoteInsideOneShellWord(t *testing.T) {
	out, err := exec.Command("sh", "-c", "printf %s "+shellQuote("a'b c")).Output()
	if err != nil || string(out) != "a'b c" {
		t.Fatalf("shellQuote must survive the shell as one word: %q %v", out, err)
	}
}

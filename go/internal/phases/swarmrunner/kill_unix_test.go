package swarmrunner

import (
	"errors"
	"os/exec"
	"syscall"
	"testing"
)

type sentSignal struct {
	pid int
	sig syscall.Signal
}

func recordingSender(sent *[]sentSignal) signalSender {
	return func(pid int, sig syscall.Signal) error {
		*sent = append(*sent, sentSignal{pid, sig})
		return nil
	}
}

func TestKillGroup_RefusesCallerInitAndNegativeGroups(t *testing.T) {
	t.Parallel()
	for _, pgid := range []int{-1, 0, initProcessGroup} {
		var sent []sentSignal
		if err := killGroup(pgid, recordingSender(&sent)); err == nil {
			t.Errorf("killGroup(%d) must be refused", pgid)
		}
		if len(sent) != 0 {
			t.Errorf("killGroup(%d) sent %v; a refused group must never be signalled", pgid, sent)
		}
	}
}

func TestKillGroup_SignalsTheWholeGroupAboveInit(t *testing.T) {
	t.Parallel()
	var sent []sentSignal
	if err := killGroup(initProcessGroup+1, recordingSender(&sent)); err != nil {
		t.Fatalf("killGroup(%d): %v", initProcessGroup+1, err)
	}
	want := sentSignal{-(initProcessGroup + 1), syscall.SIGKILL}
	if len(sent) != 1 || sent[0] != want {
		t.Fatalf("sent = %v, want %v", sent, want)
	}
}

func TestKillGroup_ReturnsTheSenderError(t *testing.T) {
	t.Parallel()
	failure := errors.New("no such process")
	err := killGroup(initProcessGroup+1, func(int, syscall.Signal) error { return failure })
	if !errors.Is(err, failure) {
		t.Fatalf("err = %v, want %v", err, failure)
	}
}

func TestGroupKiller_KillsAProcessGroupTheTestOwns(t *testing.T) {
	t.Parallel()
	child := exec.Command("sleep", "30")
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := child.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	if err := groupKiller(child.Process.Pid); err != nil {
		_ = child.Process.Kill()
		t.Fatalf("groupKiller(own child group): %v", err)
	}
	err := child.Wait()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("child wait = %v, want an exit error", err)
	}
	status, ok := exit.Sys().(syscall.WaitStatus)
	if !ok || status.Signal() != syscall.SIGKILL {
		t.Fatalf("child status = %v, want killed by SIGKILL", exit.Sys())
	}
}

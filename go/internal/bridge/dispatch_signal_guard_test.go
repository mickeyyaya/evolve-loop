package bridge

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
)

func init() {
	defaultSignalProcess = func(pid int, sig syscall.Signal) error {
		panic(fmt.Sprintf("a bridge test reached the default signal sender (%v to pid %d); inject Deps.SignalProcess", sig, pid))
	}
}

func TestWithProcessDefaults_TheDefaultSenderOfATestRefusesEverySignal(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil || !strings.Contains(fmt.Sprint(r), "inject Deps.SignalProcess") {
			t.Fatalf("recover = %v; a test that leaves SignalProcess nil must never reach kill(2)", r)
		}
	}()

	_ = Deps{}.withProcessDefaults().SignalProcess(os.Getpid(), 0)
}

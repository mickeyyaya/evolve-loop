//go:build integration

package bridge

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

type refusingWriter struct{ err error }

func (w refusingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestExecRunner_ACleanExitWhoseOutputCannotBeDeliveredIsAnUnrecoverableError(t *testing.T) {
	t.Parallel()
	sink := errors.New("stdout sink closed")
	pidFile := filepath.Join(t.TempDir(), "agent.pid")
	env := append(os.Environ(), bridgePidfileEnv+"="+pidFile)

	code, err := execRunner(itBoundedContext(t, itLaunchDeadline), "sh", "", []string{"-c", "echo agent-output"}, env, nil, refusingWriter{sink}, refusingWriter{sink})

	if code != -1 || !errors.Is(err, sink) {
		t.Fatalf("execRunner = (%d, %v), want (-1, the sink error)", code, err)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want the copy failure, not a process exit", err)
	}
	if _, statErr := os.Stat(pidFile); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("pidfile %s survived the failed wait", pidFile)
	}
}

func TestExecRunner_ANonZeroExitStillWinsOverAnUndeliverableOutput(t *testing.T) {
	t.Parallel()
	sink := errors.New("stdout sink closed")

	code, err := execRunner(itBoundedContext(t, itLaunchDeadline), "sh", "", []string{"-c", "echo agent-output; exit 5"}, nil, nil, refusingWriter{sink}, refusingWriter{sink})

	if code != 5 || err != nil {
		t.Fatalf("execRunner = (%d, %v), want the process exit (5, nil)", code, err)
	}
}

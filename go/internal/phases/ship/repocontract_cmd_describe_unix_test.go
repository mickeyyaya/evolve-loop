//go:build unix

package ship

import (
	"context"
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

type fakeWaitState struct {
	code int
	sys  any
	text string
}

func (f fakeWaitState) ExitCode() int { return f.code }
func (f fakeWaitState) Sys() any      { return f.sys }
func (f fakeWaitState) String() string {
	return f.text
}

func TestDescribeWaitResult_NamesExitStatusSignalAndElapsed(t *testing.T) {
	t.Parallel()
	var typedNilState *os.ProcessState
	cases := []struct {
		name    string
		err     error
		state   waitState
		elapsed time.Duration
		want    []string
		absent  []string
	}{
		{
			name:    "killed by SIGKILL",
			err:     errors.New("signal: killed"),
			state:   fakeWaitState{code: -1, sys: syscall.WaitStatus(syscall.SIGKILL), text: "signal: killed"},
			elapsed: 1500 * time.Millisecond,
			want:    []string{"exit=-1", "signal=killed", "elapsed=1.5s", "err=signal: killed"},
		},
		{
			name:    "clean exit after cancel",
			err:     context.Canceled,
			state:   fakeWaitState{code: 0, sys: syscall.WaitStatus(0), text: "exit status 0"},
			elapsed: 2 * time.Second,
			want:    []string{"exit=0", "signal=none", "elapsed=2s", "err=context canceled"},
			absent:  []string{"signal=killed"},
		},
		{
			name:    "non-zero exit code",
			err:     errors.New("exit status 3"),
			state:   fakeWaitState{code: 3, sys: syscall.WaitStatus(3 << 8), text: "exit status 3"},
			elapsed: 40 * time.Millisecond,
			want:    []string{"exit=3", "signal=none", "elapsed=40ms"},
			absent:  []string{"signal=killed"},
		},
		{
			name:    "terminated by SIGTERM",
			err:     errors.New("signal: terminated"),
			state:   fakeWaitState{code: -1, sys: syscall.WaitStatus(syscall.SIGTERM), text: "signal: terminated"},
			elapsed: time.Second,
			want:    []string{"exit=-1", "signal=terminated", "elapsed=1s"},
			absent:  []string{"signal=killed"},
		},
		{
			name:    "untyped nil state",
			err:     errors.New("wait: no child"),
			state:   nil,
			elapsed: 2 * time.Second,
			want:    []string{"no process state", "elapsed=2s", "err=wait: no child"},
		},
		{
			name:    "typed nil process state",
			err:     nil,
			state:   typedNilState,
			elapsed: 3 * time.Second,
			want:    []string{"no process state", "elapsed=3s", "err=<nil>"},
		},
		{
			name:    "state without a wait status",
			err:     nil,
			state:   fakeWaitState{code: 0, sys: nil, text: "exit status 0"},
			elapsed: time.Second,
			want:    []string{"exit=0", "signal=unknown", "elapsed=1s", "err=<nil>"},
			absent:  []string{"signal=none", "signal=killed"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := describeWaitResult(tc.err, tc.state, tc.elapsed)
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("describeWaitResult = %q, want it to contain %q", got, w)
				}
			}
			for _, a := range tc.absent {
				if strings.Contains(got, a) {
					t.Errorf("describeWaitResult = %q, want it not to contain %q", got, a)
				}
			}
		})
	}
}

func TestDescribeWaitResult_DistinguishesACleanExitFromAKill(t *testing.T) {
	t.Parallel()
	killed := describeWaitResult(errors.New("signal: killed"),
		fakeWaitState{code: -1, sys: syscall.WaitStatus(syscall.SIGKILL), text: "signal: killed"}, time.Second)
	clean := describeWaitResult(context.Canceled,
		fakeWaitState{code: 0, sys: syscall.WaitStatus(0), text: "exit status 0"}, time.Second)

	if killed == clean {
		t.Fatalf("a kill and a clean exit produce the same message %q", killed)
	}
	if !strings.Contains(killed, "signal=killed") || strings.Contains(clean, "signal=killed") {
		t.Errorf("signal field does not separate the two: killed=%q clean=%q", killed, clean)
	}
}

package acsassert

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

type subprocessResult struct {
	stdout, stderr string
	code           int
	err            error
}

func installFakeGo(t *testing.T, script string) string {
	t.Helper()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":/bin:/usr/bin")
	return bin
}

func setLaneEnd(t *testing.T, end time.Time) {
	t.Helper()
	t.Setenv(ipcenv.ACSLaneEndKey, end.Format(time.RFC3339Nano))
}

func callWithin(t *testing.T, guard time.Duration, name string, args ...string) subprocessResult {
	t.Helper()
	done := make(chan subprocessResult, 1)
	go func() {
		var r subprocessResult
		r.stdout, r.stderr, r.code, r.err = SubprocessOutput(name, args...)
		done <- r
	}()
	select {
	case r := <-done:
		return r
	case <-time.After(guard):
		t.Fatalf("SubprocessOutput(%s %v) did not return within %s: no bound ends a hung subprocess", name, args, guard)
		return subprocessResult{}
	}
}

func TestSubprocessOutput_ABlockingCommandReturnsInsideTheBound(t *testing.T) {
	installFakeGo(t, "exec sleep 30")
	setLaneEnd(t, time.Now().Add(time.Second))
	start := time.Now()

	r := callWithin(t, 15*time.Second, "go", "run", "./x")

	if elapsed := time.Since(start); elapsed > time.Second+6*time.Second {
		t.Errorf("returned after %s, want inside the 1s bound plus the 5s kill grace", elapsed)
	}
	if r.code == 0 || !errors.Is(r.err, ErrSubprocessBound) {
		t.Errorf("code=%d err=%v, want a nonzero code and ErrSubprocessBound", r.code, r.err)
	}
}

func TestSubprocessOutput_ALaneEndInThePastStartsNoProcess(t *testing.T) {
	bin := installFakeGo(t, "touch \"$(dirname \"$0\")/started\"")
	setLaneEnd(t, time.Now().Add(-time.Minute))

	r := callWithin(t, 5*time.Second, "go", "run", "./x")

	if _, err := os.Stat(filepath.Join(bin, "started")); err == nil {
		t.Error("the process started although the lane had ended")
	}
	if !errors.Is(r.err, ErrSubprocessBound) {
		t.Errorf("err = %v, want ErrSubprocessBound", r.err)
	}
}

func recordedArgs(t *testing.T, bin string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(bin, "args"))
	if err != nil {
		t.Fatalf("the fake go recorded no arguments: %v", err)
	}
	return strings.Fields(string(raw))
}

func TestSubprocessOutput_AGoTestGetsATimeoutBelowTheBound(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		want    string
		wantIdx int
	}{
		{"plain", []string{"test", "-count=1", "./x"}, "", 1},
		{"global_flag_first", []string{"-C", "dir", "test", "-count=1", "./x"}, "", 3},
		{"caller_timeout_kept", []string{"test", "-timeout", "7s", "./x"}, "7s", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := installFakeGo(t, "echo \"$@\" > \"$(dirname \"$0\")/args\"")
			setLaneEnd(t, time.Now().Add(100*time.Second))

			callWithin(t, 5*time.Second, "go", tc.args...)

			got := recordedArgs(t, bin)
			at := slices.Index(got, "-timeout")
			if at < 0 || at != tc.wantIdx {
				t.Fatalf("args %v: -timeout at %d, want %d", got, at, tc.wantIdx)
			}
			value, err := time.ParseDuration(got[at+1])
			if err != nil {
				t.Fatalf("-timeout value %q: %v", got[at+1], err)
			}
			if tc.want != "" {
				if got[at+1] != tc.want {
					t.Errorf("caller -timeout became %q, want %q", got[at+1], tc.want)
				}
				return
			}
			if value < 85*time.Second || value > 91*time.Second {
				t.Errorf("-timeout %s, want about 0.9 of the 100s bound", value)
			}
		})
	}
}

func TestSubprocessOutput_ANonGoCommandGetsNoTimeoutFlag(t *testing.T) {
	stdout, _, code, err := SubprocessOutput("echo", "test", "x")
	if err != nil || code != 0 || strings.TrimSpace(stdout) != "test x" {
		t.Errorf("echo test x = %q code=%d err=%v, want the arguments untouched", stdout, code, err)
	}
}

func TestSubprocessOutput_ABoundHitNamesTheBound(t *testing.T) {
	t.Run("deadline_kill", func(t *testing.T) {
		installFakeGo(t, "exec sleep 30")
		setLaneEnd(t, time.Now().Add(500*time.Millisecond))

		r := callWithin(t, 15*time.Second, "go", "run", "./x")

		if !errors.Is(r.err, ErrSubprocessBound) || !strings.Contains(r.stderr, acsverdict.SubprocessBoundMarker) {
			t.Errorf("err=%v stderr=%q, want the sentinel and the marker line a predicate prints", r.err, r.stderr)
		}
	})
	t.Run("inner_go_test_timeout", func(t *testing.T) {
		installFakeGo(t, "echo 'panic: test timed out after 1s'; exit 2")

		r := callWithin(t, 5*time.Second, "go", "test", "./x")

		if r.code != 2 || !errors.Is(r.err, ErrSubprocessBound) || !strings.Contains(r.stderr, acsverdict.SubprocessBoundMarker) {
			t.Errorf("code=%d err=%v stderr=%q, want the go test exit code kept and the bound named", r.code, r.err, r.stderr)
		}
	})
	t.Run("cap_without_a_lane_end", func(t *testing.T) {
		installFakeGo(t, "exec sleep 30")
		t.Setenv(ipcenv.ACSLaneEndKey, "")
		old := subprocessCap
		subprocessCap = time.Second
		t.Cleanup(func() { subprocessCap = old })

		r := callWithin(t, 15*time.Second, "go", "run", "./x")

		if !errors.Is(r.err, ErrSubprocessBound) {
			t.Errorf("err = %v, want the cap to bound a call with no lane end", r.err)
		}
	})
	t.Run("an_ordinary_failure_is_not_a_bound_hit", func(t *testing.T) {
		installFakeGo(t, "echo FAIL; exit 1")
		setLaneEnd(t, time.Now().Add(time.Minute))

		r := callWithin(t, 5*time.Second, "go", "test", "./x")

		if r.code != 1 || errors.Is(r.err, ErrSubprocessBound) || strings.Contains(r.stderr, acsverdict.SubprocessBoundMarker) {
			t.Errorf("code=%d err=%v stderr=%q, want a plain exit 1", r.code, r.err, r.stderr)
		}
	})
}

func TestSubprocessOutput_AMalformedLaneEndFallsBackToTheCap(t *testing.T) {
	installFakeGo(t, "exec sleep 30")
	t.Setenv(ipcenv.ACSLaneEndKey, "not-a-time")
	old := subprocessCap
	subprocessCap = time.Second
	t.Cleanup(func() { subprocessCap = old })

	r := callWithin(t, 15*time.Second, "go", "run", "./x")

	if !errors.Is(r.err, ErrSubprocessBound) {
		t.Errorf("err = %v, want the cap to still bound the call", r.err)
	}
}

type recordingT struct{ messages []string }

func (r *recordingT) Errorf(format string, args ...any) {
	r.messages = append(r.messages, fmt.Sprintf(format, args...))
}
func (r *recordingT) Helper() {}

type boundCallerT struct {
	recordingT
	deadline time.Time
}

func (d *boundCallerT) Deadline() (time.Time, bool) { return d.deadline, true }

func TestGoTests_ABlockingGoTestReturnsInsideTheBound(t *testing.T) {
	installFakeGo(t, "exec sleep 30")
	setLaneEnd(t, time.Now().Add(time.Second))
	tb := &recordingT{}
	done := make(chan bool, 1)
	go func() {
		done <- GoTests(tb, GoTestSpec{Dir: t.TempDir(), Package: "./x", Pattern: "^TestA$", Names: []string{"TestA"}})
	}()

	select {
	case ok := <-done:
		if ok || !strings.Contains(strings.Join(tb.messages, "\n"), acsverdict.SubprocessBoundMarker) {
			t.Errorf("ok=%v errors=%v, want a failure that names the bound", ok, tb.messages)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("GoTests did not return: the lane end does not bound go list or go test")
	}
}

func TestGoTests_TheCallerDeadlineStillBoundsWhenEarlierThanTheLane(t *testing.T) {
	installFakeGo(t, "exec sleep 30")
	setLaneEnd(t, time.Now().Add(time.Hour))
	tb := &boundCallerT{deadline: time.Now().Add(time.Second)}
	done := make(chan bool, 1)
	go func() {
		done <- GoTests(tb, GoTestSpec{Dir: t.TempDir(), Package: "./x", Pattern: "^TestA$", Names: []string{"TestA"}})
	}()

	select {
	case ok := <-done:
		if ok {
			t.Error("GoTests reported success for a hung go test")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("GoTests ignored the earlier caller deadline")
	}
}

func TestGoTests_AHungGoTestStepNamesTheBound(t *testing.T) {
	installFakeGo(t, "if [ \"$1\" = list ]; then echo example.com/x; exit 0; fi\nexec sleep 30")
	setLaneEnd(t, time.Now().Add(time.Second))
	tb := &recordingT{}
	done := make(chan bool, 1)
	go func() {
		done <- GoTests(tb, GoTestSpec{Dir: t.TempDir(), Package: "./x", Pattern: "^TestA$", Names: []string{"TestA"}})
	}()

	select {
	case ok := <-done:
		if ok || !strings.Contains(strings.Join(tb.messages, "\n"), acsverdict.SubprocessBoundMarker) {
			t.Errorf("ok=%v errors=%v, want the go test step to name the bound so the lane does not retry", ok, tb.messages)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("GoTests did not return: the lane end does not bound the go test step")
	}
}

func TestSubprocessOutputIn_RunsTheCommandInTheGivenDirectory(t *testing.T) {
	dir := t.TempDir()
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	stdout, _, code, err := SubprocessOutputIn(dir, "pwd")
	if err != nil || code != 0 {
		t.Fatalf("pwd in %s: code %d err %v", dir, code, err)
	}
	if got, _ := filepath.EvalSymlinks(strings.TrimSpace(stdout)); got != want {
		t.Errorf("pwd printed %q, want %q", got, want)
	}
}

func TestSubprocessOutput_ANonGoTimeoutTextIsNotABoundHit(t *testing.T) {
	installFakeGo(t, "echo 'panic: test timed out after 1s'; exit 2")
	setLaneEnd(t, time.Now().Add(time.Minute))

	_, _, code, err := SubprocessOutput("sh", "-c", "go")

	if code != 2 || errors.Is(err, ErrSubprocessBound) {
		t.Errorf("code=%d err=%v, want a plain exit 2 and no bound hit for a command that is not go", code, err)
	}
}

package bridge

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestPidFileFor(t *testing.T) {
	cases := map[string]string{
		"/ws/build-stdout.log": "/ws/build.bridge-pid",
		"/ws/tdd-stdout.log":   "/ws/tdd.bridge-pid",
		"build-stdout.log":     "build.bridge-pid",
		"":                     "",
		"/ws/build.log":        "", // not the -stdout.log convention
	}
	for in, want := range cases {
		if got := pidFileFor(in); got != want {
			t.Errorf("pidFileFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEnvValue(t *testing.T) {
	env := []string{"A=1", "B=two", "X=a=b"}
	cases := map[string]string{"A": "1", "B": "two", "X": "a=b", "MISSING": ""}
	for k, want := range cases {
		if got := envValue(env, k); got != want {
			t.Errorf("envValue(%q) = %q, want %q", k, got, want)
		}
	}
}

// TestExecRunner_WritesPIDFile verifies the PID file is written for the child to
// read (the child cats it into OUT, which persists) and removed after exit.
func TestExecRunner_WritesPIDFile(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "x.bridge-pid")
	out := filepath.Join(dir, "out")
	env := append(os.Environ(), "EVOLVE_BRIDGE_PIDFILE="+pidFile, "OUT="+out)

	rc, err := execRunner(context.Background(), "sh", "",
		[]string{"-c", `cat "$EVOLVE_BRIDGE_PIDFILE" > "$OUT"`},
		env, nil, io.Discard, io.Discard)
	if err != nil || rc != 0 {
		t.Fatalf("execRunner rc=%d err=%v", rc, err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read OUT: %v", err)
	}
	if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err != nil || pid <= 0 {
		t.Fatalf("pidfile content %q is not a valid pid", data)
	}
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Errorf("pidfile should be removed after exit; stat err=%v", err)
	}
}

// TestExecRunner_NoPIDFileEnv_NoWrite confirms the gating: without
// EVOLVE_BRIDGE_PIDFILE, no pid file is created (byte-identical to the prior
// behavior).
func TestExecRunner_NoPIDFileEnv_NoWrite(t *testing.T) {
	dir := t.TempDir()
	stray := filepath.Join(dir, "should-not-exist.bridge-pid")
	rc, err := execRunner(context.Background(), "sh", "", []string{"-c", "true"},
		os.Environ(), nil, io.Discard, io.Discard)
	if err != nil || rc != 0 {
		t.Fatalf("execRunner rc=%d err=%v", rc, err)
	}
	if _, err := os.Stat(stray); !os.IsNotExist(err) {
		t.Errorf("no pid file should be created without the env gate")
	}
}

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

func TestEnvValue(t *testing.T) {
	env := []string{"A=1", "B=two", "X=a=b"}
	cases := map[string]string{"A": "1", "B": "two", "X": "a=b", "MISSING": ""}
	for k, want := range cases {
		if got := envValue(env, k); got != want {
			t.Errorf("envValue(%q) = %q, want %q", k, got, want)
		}
	}
}

func TestExecRunner_WritesPIDFile(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "x.bridge-pid")
	out := filepath.Join(dir, "out")
	env := append(os.Environ(), "EVOLVE_BRIDGE_PIDFILE="+pidFile, "OUT="+out)

	rc, err := execRunner(context.Background(), "sh", "",
		[]string{"-c", `i=0; while [ $i -lt 100 ]; do [ -s "$EVOLVE_BRIDGE_PIDFILE" ] && break; sleep 0.02; i=$((i+1)); done; cat "$EVOLVE_BRIDGE_PIDFILE" > "$OUT"`},
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

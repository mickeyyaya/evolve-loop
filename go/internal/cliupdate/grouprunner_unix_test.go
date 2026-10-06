//go:build unix

package cliupdate_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliupdate"
)

func TestGroupRunner_ADeadlineKillsTheUpdatersWholeProcessGroup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var out bytes.Buffer
	start := time.Now()

	code, err := cliupdate.GroupRunner(ctx, "sh", "", []string{"-c", "sleep 12 & sleep 60"}, nil, nil, &out, &out)

	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("a grandchild holding the output must not outlive the deadline: returned after %s", elapsed.Round(time.Millisecond))
	}
	if code != -1 || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a killed updater reports the deadline, never a clean exit: code=%d err=%v", code, err)
	}
}

func TestGroupRunner_ReportsTheExitCodeAndTheOutput(t *testing.T) {
	var out bytes.Buffer

	code, err := cliupdate.GroupRunner(context.Background(), "sh", "", []string{"-c", "echo $CLIUPDATE_ENV_PROBE; exit 3"}, []string{"CLIUPDATE_ENV_PROBE=from-env"}, nil, &out, &out)

	if code != 3 || err != nil || strings.TrimSpace(out.String()) != "from-env" {
		t.Fatalf("want exit 3 with the given env, got code=%d err=%v out=%q", code, err, out.String())
	}
	if code, err := cliupdate.GroupRunner(context.Background(), "true", "", nil, nil, nil, &out, &out); code != 0 || err != nil {
		t.Fatalf("a clean exit is (0, nil), got (%d, %v)", code, err)
	}
}

func TestGroupRunner_ALaunchFailureIsAnError(t *testing.T) {
	var out bytes.Buffer

	code, err := cliupdate.GroupRunner(context.Background(), "/nonexistent/updater", "", nil, nil, nil, &out, &out)

	if code != -1 || err == nil {
		t.Fatalf("a binary that cannot start is (-1, err), got (%d, %v)", code, err)
	}
}

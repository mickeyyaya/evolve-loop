//go:build integration

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func fakeCycleBinary(t *testing.T, body string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "evolve")
	fakeclitest.Install(t, bin, "#!/bin/sh\n"+body)
	return bin
}

func TestExecCycleLaunch_RunsTheChildWithLaneArgsEnvAndPrefixedOutput(t *testing.T) {
	t.Parallel()
	bin := fakeCycleBinary(t, "echo \"args:$*\"\necho \"env:$LANE_MARK\"\necho oops >&2\nexit 3\n")
	var stdout, stderr bytes.Buffer
	launch := execCycleLaunch(bin, true, "/proj", "fallbackhash", "the goal", &stdout, &stderr)

	code, err := launch(context.Background(), fleet.CycleSpec{GoalHash: "abcdef0123", Scope: []string{"t1", "t2"}, Env: map[string]string{"LANE_MARK": "lane-7"}})

	if err != nil || code != 3 {
		t.Fatalf("= (%d, %v), want the child's exit 3 and no error", code, err)
	}
	wantOut := "[t1+t2] args:cycle run --goal-hash abcdef0123 --goal the goal -simulate --project-root /proj\n[t1+t2] env:lane-7\n"
	if stdout.String() != wantOut {
		t.Errorf("stdout = %q, want %q", stdout.String(), wantOut)
	}
	if stderr.String() != "[t1+t2] oops\n" {
		t.Errorf("stderr = %q, want the prefixed child stderr", stderr.String())
	}
}

func TestExecCycleLaunch_TheFallbackHashAndContractApplyWhenTheSpecHasNone(t *testing.T) {
	t.Parallel()
	bin := fakeCycleBinary(t, "echo \"$*\"\n")
	var stdout bytes.Buffer
	launch := execCycleLaunch(bin, false, "", "fallbackhash", "goal text", &stdout, &bytes.Buffer{})

	code, err := launch(context.Background(), fleet.CycleSpec{OutputContract: "the contract"})

	if err != nil || code != 0 {
		t.Fatalf("= (%d, %v), want (0, nil)", code, err)
	}
	if got := stdout.String(); got != "[] cycle run --goal-hash fallbackhash --goal the contract\n" {
		t.Errorf("stdout = %q, want the fallback hash and the contract as goal", got)
	}
}

func TestExecCycleLaunch_AMissingBinaryIsAnError(t *testing.T) {
	t.Parallel()
	launch := execCycleLaunch(filepath.Join(t.TempDir(), "absent"), false, "", "h", "", &bytes.Buffer{}, &bytes.Buffer{})

	code, err := launch(context.Background(), fleet.CycleSpec{GoalHash: "h"})

	if err == nil || code != -1 {
		t.Errorf("= (%d, %v), want (-1, a start error)", code, err)
	}
}

func TestExecCycleLaunch_CancelTerminatesTheChild(t *testing.T) {
	t.Parallel()
	bin := fakeCycleBinary(t, "trap 'echo got-term; exit 0' TERM\n: > \"$READY\"\nwhile :; do sleep 0.05; done\n")
	ctx, cancel := context.WithCancel(context.Background())
	var stdout bytes.Buffer
	launch := execCycleLaunch(bin, false, "", "h", "", &stdout, &bytes.Buffer{})
	ready := filepath.Join(t.TempDir(), "ready")
	done := make(chan struct{})
	var code int
	var err error
	go func() {
		code, err = launch(ctx, fleet.CycleSpec{GoalHash: "h", Env: map[string]string{"READY": ready}})
		close(done)
	}()
	deadline := time.Now().Add(10 * time.Second)
	for _, statErr := os.Stat(ready); statErr != nil; _, statErr = os.Stat(ready) {
		if time.Now().After(deadline) {
			t.Fatal("child never installed its TERM trap")
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()

	select {
	case <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("launch did not return within 8s of cancel; SIGTERM was not delivered")
	}
	if !errors.Is(err, context.Canceled) || code != -1 {
		t.Errorf("= (%d, %v), want (-1, context.Canceled): a cancelled lane is an error, not an exit code", code, err)
	}
	if !strings.Contains(stdout.String(), "[h] got-term") {
		t.Errorf("stdout=%q, want the child to receive SIGTERM (a graceful stop), not SIGKILL", stdout.String())
	}
}

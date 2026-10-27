package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func waveStopRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func liveWaveLease(t *testing.T, root string, at time.Time) string {
	t.Helper()
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-41")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "01LIVE", OwnerPID: os.Getpid()}, at); err != nil {
		t.Fatal(err)
	}
	return runDir
}

func TestWaveStop_EngagesTheBrakeAndPrintsTheResult(t *testing.T) {
	t.Parallel()
	root := waveStopRoot(t)
	h := newWaveHarness()
	rc := runWaveWith(h.env(root), []string{"stop", "--json", "--project-root", root}, &h.stdout, &h.stderr)
	if rc != cliExitOK {
		t.Fatalf("rc = %d stderr = %s", rc, h.stderr.String())
	}
	brake := paths.LoopStopPath(filepath.Join(root, ".evolve"))
	if _, err := os.Stat(brake); err != nil {
		t.Errorf("no brake at %s: %v", brake, err)
	}
	var got loopStopResult
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not one result: %v\n%s", err, h.stdout.String())
	}
	if got.Brake != brake || got.Action != "engaged" || got.Idle || h.sleeps != 0 {
		t.Errorf("result = %+v sleeps = %d, want an engaged brake and no wait", got, h.sleeps)
	}
}

func TestWaveStop_WaitBlocksUntilNoRunLeaseIsLive(t *testing.T) {
	t.Parallel()
	root := waveStopRoot(t)
	runDir := liveWaveLease(t, root, waveTestClock)
	h := newWaveHarness()
	h.onSleep = func(int) { _ = os.RemoveAll(runDir) }
	rc := runWaveWith(h.env(root), []string{"stop", "--wait", "10m", "--project-root", root}, &h.stdout, &h.stderr)
	if rc != cliExitOK || h.sleeps != 1 {
		t.Fatalf("rc = %d sleeps = %d stderr = %s, want 0 after one wait", rc, h.sleeps, h.stderr.String())
	}
	if out := h.stdout.String(); !strings.Contains(out, "waiting on run 01LIVE") || !strings.Contains(out, "the loop is idle") {
		t.Errorf("stdout = %q", out)
	}
}

func TestWaveStop_AWaitPastItsTimeKeepsTheBrakeAndFails(t *testing.T) {
	t.Parallel()
	root := waveStopRoot(t)
	liveWaveLease(t, root, waveTestClock)
	h := newWaveHarness()
	clock := waveTestClock
	env := h.env(root)
	env.now = func() time.Time { return clock }
	h.onSleep = func(int) { clock = clock.Add(2 * time.Minute) }
	rc := runWaveWith(env, []string{"stop", "--wait", "1m", "--json", "--project-root", root}, &h.stdout, &h.stderr)
	if rc != cliExitFailed || !strings.Contains(h.stderr.String(), "timed out after 1m0s") || h.stdout.Len() != 0 {
		t.Errorf("rc = %d stdout = %q stderr = %q, want 1, the timeout and no result", rc, h.stdout.String(), h.stderr.String())
	}
	if _, err := os.Stat(paths.LoopStopPath(filepath.Join(root, ".evolve"))); err != nil {
		t.Errorf("the brake is gone after the timeout: %v", err)
	}
}

func TestWaveStop_EachFaultHasItsExitCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
		prep func(h *waveHarness, root string)
		want int
	}{
		{"a stray word", []string{"stop", "now"}, nil, cliExitUsage},
		{"an unknown flag", []string{"stop", "--bogus"}, nil, cliExitUsage},
		{"a wait that is not a duration", []string{"stop", "--wait", "soon"}, nil, cliExitUsage},
		{"a negative wait", []string{"stop", "--wait", "-1m"}, nil, cliExitUsage},
		{"no root", []string{"stop"}, func(h *waveHarness, _ string) { h.rootErr = errors.New("getwd: gone") }, cliExitIO},
		{"no evolve directory", []string{"stop"}, func(_ *waveHarness, root string) { _ = os.RemoveAll(filepath.Join(root, ".evolve")) }, cliExitIO},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := waveStopRoot(t)
			h := newWaveHarness()
			if c.prep != nil {
				c.prep(h, root)
			}
			if rc := runWaveWith(h.env(root), append(c.args, "--project-root", root), &h.stdout, &h.stderr); rc != c.want || h.stderr.Len() == 0 {
				t.Errorf("rc = %d stderr = %q, want %d and a message", rc, h.stderr.String(), c.want)
			}
		})
	}
}

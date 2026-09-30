package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func TestPRCheckState_ClassifiesTheRollup(t *testing.T) {
	cases := []struct {
		name        string
		checks      []ghPRCheck
		wantState   string
		wantFailing []string
	}{
		{"no-checks", nil, "none", nil},
		{"all-green", []ghPRCheck{{Name: "unit", Status: "COMPLETED", Conclusion: "SUCCESS"}, {Context: "ci/legacy", State: "SUCCESS"}}, "passing", nil},
		{"in-progress", []ghPRCheck{{Name: "unit", Status: "IN_PROGRESS"}, {Name: "lint", Status: "COMPLETED", Conclusion: "SUCCESS"}}, "pending", nil},
		{"status-context-pending", []ghPRCheck{{Context: "ci/legacy", State: "PENDING"}}, "pending", nil},
		{"failure-wins-over-pending", []ghPRCheck{{Name: "unit", Status: "IN_PROGRESS"}, {Name: "scan", Status: "COMPLETED", Conclusion: "FAILURE"}, {Context: "ci/legacy", State: "ERROR"}}, "failing", []string{"scan", "ci/legacy"}},
	}
	for _, c := range cases {
		state, failing := prCheckState(c.checks)
		if state != c.wantState || strings.Join(failing, ",") != strings.Join(c.wantFailing, ",") {
			t.Errorf("%s: got %q %v, want %q %v", c.name, state, failing, c.wantState, c.wantFailing)
		}
	}
}

func TestStatusSnapshotReadable_RefusesAMissingOrNonDirectoryEvolveDir(t *testing.T) {
	ok := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ok, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	notDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(notDir, ".evolve"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := statusSnapshotReadable(ok); err != nil {
		t.Errorf("a project with .evolve/ must be readable: %v", err)
	}
	for _, root := range []string{filepath.Join(t.TempDir(), "missing"), notDir} {
		if err := statusSnapshotReadable(root); err == nil {
			t.Errorf("%s must be unreadable", root)
		}
	}
}

func TestWaitForIdleLoop_PrintsTheRunPhaseOnlyWhenItChanges(t *testing.T) {
	runs := t.TempDir()
	dir := filepath.Join(runs, "cycle-9")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	setPhase := func(phase string) {
		if err := os.WriteFile(filepath.Join(dir, "run.json"), []byte(`{"phase":"`+phase+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	setPhase("build")
	if err := runlease.Write(dir, runlease.Lease{RunID: "run-x", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	clock := time.Now()
	ticks := 0
	now := func() time.Time { return clock }
	sleep := func(d time.Duration) {
		clock = clock.Add(d)
		ticks++
		switch ticks {
		case 2:
			setPhase("audit")
		case 4:
			if err := os.Remove(runlease.PathIn(dir)); err != nil {
				t.Fatal(err)
			}
		}
	}
	var out, errb bytes.Buffer
	if rc := waitForIdleLoop(runs, time.Minute, time.Second, now, sleep, &out, &errb); rc != 0 {
		t.Fatalf("rc=%d want 0; stderr %q", rc, errb.String())
	}
	want := "loop-stop: waiting on run run-x (cycle-9) in phase build\nloop-stop: waiting on run run-x (cycle-9) in phase audit\nloop-stop: no run lease is live; the loop is idle\n"
	if out.String() != want {
		t.Fatalf("stdout =\n%s\nwant\n%s", out.String(), want)
	}
}

func TestWaitForIdleLoop_TimesOutNamingLiveRun(t *testing.T) {
	runs := t.TempDir()
	dir := filepath.Join(runs, "cycle-9")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(dir, runlease.Lease{RunID: "run-x", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	clock := time.Now()
	now := func() time.Time { return clock }
	sleep := func(d time.Duration) { clock = clock.Add(d) }
	var out, errb bytes.Buffer
	if rc := waitForIdleLoop(runs, time.Second, 200*time.Millisecond, now, sleep, &out, &errb); rc != 1 {
		t.Fatalf("rc=%d want 1", rc)
	}
	if !strings.Contains(errb.String(), "run-x") {
		t.Fatalf("timeout must name the run; got %q", errb.String())
	}
	if err := os.Remove(runlease.PathIn(dir)); err != nil {
		t.Fatal(err)
	}
	if rc := waitForIdleLoop(runs, time.Second, 200*time.Millisecond, now, sleep, &out, &errb); rc != 0 {
		t.Fatalf("idle: rc=%d want 0", rc)
	}
}

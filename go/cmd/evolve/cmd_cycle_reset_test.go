package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// cmd_cycle_reset_test.go — F2/F5: `evolve cycle reset` must consult the run
// lease (via SealCycle) and, on a LIVE owner, refuse with an actionable message
// that names the owner and steers to --resume / a clean SIGTERM — never to
// `pkill`. --force overrides a live owner with a loud WARN; a stale/absent
// lease seals normally.

// seedReset writes <root>/.evolve/{cycle-state.json,state.json,runs/cycle-N/...}.
func seedReset(t *testing.T, root string, cycleID int) (evolveDir, workspace string) {
	t.Helper()
	evolveDir = filepath.Join(root, ".evolve")
	workspace = filepath.Join(evolveDir, "runs", fmt.Sprintf("cycle-%d", cycleID))
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "build-report.md"), []byte("partial\n"), 0o644); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	cs := fmt.Sprintf(`{"cycle_id":%d,"phase":"build","workspace_path":%q}`, cycleID, workspace)
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), []byte(cs), 0o644); err != nil {
		t.Fatalf("write cycle-state: %v", err)
	}
	st := fmt.Sprintf(`{"lastCycleNumber":%d}`, cycleID-1)
	if err := os.WriteFile(filepath.Join(evolveDir, "state.json"), []byte(st), 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}
	return evolveDir, workspace
}

func TestRunCycleReset_LeaseFencing(t *testing.T) {
	// A genuinely-alive owner pid: cycle-554's PID-aware fence (runlease.OwnerLive,
	// wired via SealOptions.PidAlive) now demands BOTH a fresh heartbeat AND a
	// live process to treat a cycle as owned. This test process's own pid is a
	// guaranteed-live owner; an arbitrary pid (84055) is almost certainly dead
	// and would now correctly seal — see the dead-owner subtest below.
	livePID := os.Getpid()

	t.Run("fresh lease with a live owner refuses with owner message", func(t *testing.T) {
		root := t.TempDir()
		ev, ws := seedReset(t, root, 395)
		if err := runlease.Write(ws, runlease.Lease{RunID: "01RUN9", OwnerPID: livePID}, time.Now()); err != nil {
			t.Fatalf("write lease: %v", err)
		}
		var out, errB bytes.Buffer
		rc := runCycleReset([]string{"--project-root", root, "--evolve-dir", ev}, &out, &errB)
		if rc != 1 {
			t.Fatalf("rc=%d, want 1 (refuse live owner); stderr=%s", rc, errB.String())
		}
		s := errB.String()
		for _, want := range []string{"LIVE", strconv.Itoa(livePID), "evolve loop --resume"} {
			if !strings.Contains(s, want) {
				t.Errorf("stderr missing %q; got:\n%s", want, s)
			}
		}
		if _, err := os.Stat(filepath.Join(ev, "cycle-state.json")); err != nil {
			t.Errorf("cycle-state must survive the refusal: %v", err)
		}
	})

	t.Run("dead owner with a fresh lease seals WITHOUT --force", func(t *testing.T) {
		// The cycle-554 fix's payoff at the CLI level: a crashed owner whose
		// heartbeat is still fresh (the 2-6min post-crash window) used to force
		// `evolve cycle reset --force` at every batch boundary; now a plain reset
		// seals it because the owning pid is dead. Exercises the cmd_cycle.go
		// PidAlive wiring end to end.
		root := t.TempDir()
		ev, ws := seedReset(t, root, 395)
		if err := runlease.Write(ws, runlease.Lease{RunID: "01RUN9", OwnerPID: 999999}, time.Now()); err != nil {
			t.Fatalf("write lease: %v", err)
		}
		var out, errB bytes.Buffer
		rc := runCycleReset([]string{"--project-root", root, "--evolve-dir", ev}, &out, &errB)
		if rc != 0 {
			t.Fatalf("rc=%d, want 0 (dead-owner fresh lease seals without --force); stderr=%s", rc, errB.String())
		}
		if !strings.Contains(out.String(), "sealed cycle 395") {
			t.Errorf("expected seal confirmation on stdout; got:\n%s", out.String())
		}
	})

	t.Run("--force overrides a live-owner fresh lease with a loud WARN", func(t *testing.T) {
		root := t.TempDir()
		ev, ws := seedReset(t, root, 395)
		if err := runlease.Write(ws, runlease.Lease{RunID: "01RUN9", OwnerPID: livePID}, time.Now()); err != nil {
			t.Fatalf("write lease: %v", err)
		}
		var out, errB bytes.Buffer
		rc := runCycleReset([]string{"--project-root", root, "--evolve-dir", ev, "--force"}, &out, &errB)
		if rc != 0 {
			t.Fatalf("rc=%d, want 0 (--force seals); stderr=%s", rc, errB.String())
		}
		if !strings.Contains(errB.String(), "WARN") || !strings.Contains(errB.String(), "FRESH") {
			t.Errorf("--force over a live lease must warn loudly; got:\n%s", errB.String())
		}
	})

	t.Run("stale lease seals without --force", func(t *testing.T) {
		root := t.TempDir()
		ev, ws := seedReset(t, root, 395)
		if err := runlease.Write(ws, runlease.Lease{RunID: "01RUN9", OwnerPID: 84055}, time.Now().Add(-20*time.Minute)); err != nil {
			t.Fatalf("write lease: %v", err)
		}
		var out, errB bytes.Buffer
		rc := runCycleReset([]string{"--project-root", root, "--evolve-dir", ev}, &out, &errB)
		if rc != 0 {
			t.Fatalf("rc=%d, want 0 (stale lease auto-seals); stderr=%s", rc, errB.String())
		}
		if !strings.Contains(out.String(), "sealed cycle 395") {
			t.Errorf("expected seal confirmation on stdout; got:\n%s", out.String())
		}
	})
}

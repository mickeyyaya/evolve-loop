package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func runRetire(args ...string) (rc int, stdout, stderr string) {
	var out, errb bytes.Buffer
	rc = runCycle(append([]string{"retire"}, args...), nil, &out, &errb)
	return rc, out.String(), errb.String()
}

func TestCycleRetire_RetiresAPausedCycleAndRecordsTheReason(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	seedPausedRun(t, root, 3, 0, 0, "item-a")

	rc, stdout, stderr := runRetire("--cycle", "3", "--reason", "abandoned", "--project-root", root)

	if rc != cliExitOK || !strings.Contains(stdout, "retired cycle 3") {
		t.Fatalf("retire = rc %d stdout %q stderr %q, want ok", rc, stdout, stderr)
	}
	ledgerRaw, _ := os.ReadFile(filepath.Join(root, ".evolve", "ledger.jsonl"))
	if !strings.Contains(string(ledgerRaw), `"kind":"retire"`) || !strings.Contains(string(ledgerRaw), "abandoned") {
		t.Errorf("ledger = %s, want a retire entry with the reason", ledgerRaw)
	}
	rc, _, stderr = runRetire("--cycle", "3", "--project-root", root)
	if rc != cliExitInput || !strings.Contains(stderr, "not a paused cycle") {
		t.Errorf("second retire = rc %d stderr %q, want %d not paused", rc, stderr, cliExitInput)
	}
}

func TestCycleRetire_JSONNamesTheCycle(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	seedPausedRun(t, root, 4, 0, 0, "item-b")

	rc, stdout, stderr := runRetire("--cycle", "4", "--json", "--project-root", root)

	if rc != cliExitOK || !strings.Contains(stdout, `"cycle": 4`) && !strings.Contains(stdout, `"cycle":4`) {
		t.Errorf("retire --json = rc %d stdout %q stderr %q", rc, stdout, stderr)
	}
}

func TestCycleRetire_RefusesALiveLaneAndAnUsageError(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	statePath := seedPausedRun(t, root, 5, 0, 0, "item-c")
	if err := runlease.Write(filepath.Dir(statePath), runlease.Lease{RunID: "live", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}

	if rc, _, stderr := runRetire("--cycle", "5", "--project-root", root); rc != cliExitRefused || !strings.Contains(stderr, "live lane") {
		t.Errorf("live lane = rc %d stderr %q, want %d", rc, stderr, cliExitRefused)
	}
	if rc, _, _ := runRetire("--project-root", root); rc != cliExitUsage {
		t.Errorf("no --cycle = rc %d, want %d", rc, cliExitUsage)
	}
}

func TestCycleRetire_ReportsAnUnreadableRunsDirectory(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	if err := os.WriteFile(filepath.Join(root, ".evolve", "runs"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if rc, _, stderr := runRetire("--cycle", "1", "--project-root", root); rc != cliExitIO || !strings.Contains(stderr, "cannot list") {
		t.Errorf("unreadable runs = rc %d stderr %q, want %d", rc, stderr, cliExitIO)
	}
}

func TestCycleRetire_ReportsAMissingWorkingDirectory(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	saved := processGetwd
	processGetwd = func() (string, error) { return "", errors.New("gone") }
	t.Cleanup(func() { processGetwd = saved })

	if rc, _, stderr := runRetire("--cycle", "1"); rc != cliExitIO || !strings.Contains(stderr, "cwd") {
		t.Errorf("no cwd = rc %d stderr %q, want %d", rc, stderr, cliExitIO)
	}
}

func TestRetirePausedCycle_FailureModes(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer

	t.Run("ledger", func(t *testing.T) {
		root := newLoopStatusRoot(t)
		path := seedPausedRun(t, root, 6, 0, 0, "item-d")
		if err := os.Mkdir(filepath.Join(root, ".evolve", "ledger.jsonl"), 0o755); err != nil {
			t.Fatal(err)
		}
		_, rc := retirePausedCycle(root, filepath.Join(root, ".evolve"), core.PausedScope{Cycle: 6, StatePath: path}, "r", nil, &stderr)
		if rc != cliExitFailed {
			t.Errorf("ledger failure rc = %d, want %d", rc, cliExitFailed)
		}
	})
	t.Run("revoke", func(t *testing.T) {
		root := newLoopStatusRoot(t)
		_, rc := retirePausedCycle(root, filepath.Join(root, ".evolve"), core.PausedScope{Cycle: 7, StatePath: filepath.Join(root, "absent.json")}, "r", nil, &stderr)
		if rc != cliExitFailed || !strings.Contains(stderr.String(), "revoke the resume grant") {
			t.Errorf("revoke failure rc = %d stderr %q, want %d", rc, stderr.String(), cliExitFailed)
		}
	})
	t.Run("release", func(t *testing.T) {
		root := newLoopStatusRoot(t)
		path := seedPausedRun(t, root, 8, 0, 0, "item-e")
		if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox", "processing"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		stderr.Reset()
		res, rc := retirePausedCycle(root, filepath.Join(root, ".evolve"), core.PausedScope{Cycle: 8, StatePath: path}, "r", nil, &stderr)
		if rc != cliExitOK || res.Released != 0 || !strings.Contains(stderr.String(), "claims were not released") {
			t.Errorf("release failure = rc %d res %+v stderr %q, want ok with a WARN", rc, res, stderr.String())
		}
	})
}

func TestCycleRetire_ALedgerFailureFailsTheVerb(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	seedPausedRun(t, root, 9, 0, 0, "item-f")
	if err := os.Mkdir(filepath.Join(root, ".evolve", "ledger.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}

	if rc, _, stderr := runRetire("--cycle", "9", "--project-root", root); rc != cliExitFailed || !strings.Contains(stderr, "ledger") {
		t.Errorf("ledger failure = rc %d stderr %q, want %d", rc, stderr, cliExitFailed)
	}
}

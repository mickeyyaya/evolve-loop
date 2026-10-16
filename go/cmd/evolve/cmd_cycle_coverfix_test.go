package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCycleResetCoverfixBadFlagExits10(t *testing.T) {
	t.Parallel()
	var out, errB bytes.Buffer
	if rc := runCycleReset([]string{"--no-such-flag"}, &out, &errB); rc != 10 {
		t.Fatalf("rc=%d, want 10", rc)
	}
}

func TestCycleResetCoverfixNothingToResetExits1(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var out, errB bytes.Buffer
	rc := runCycleReset([]string{"--project-root", root, "--evolve-dir", filepath.Join(root, ".evolve")}, &out, &errB)
	if rc != 1 {
		t.Fatalf("rc=%d, want 1; stderr=%s", rc, errB.String())
	}
	if !strings.Contains(errB.String(), "no in-progress cycle to seal") {
		t.Errorf("stderr=%q", errB.String())
	}
}

func TestCycleResetCoverfixUnreadableStateReportsGenericError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ev := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(ev, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ev, "cycle-state.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errB bytes.Buffer
	rc := runCycleReset([]string{"--project-root", root, "--evolve-dir", ev}, &out, &errB)
	if rc != 1 {
		t.Fatalf("rc=%d, want 1", rc)
	}
	if !strings.Contains(errB.String(), "evolve cycle reset: reset: parse cycle-state") {
		t.Errorf("stderr=%q, want the generic error path", errB.String())
	}
}

func TestCycleResetCoverfixDefaultEvolveDirFromProjectRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, _ = seedReset(t, root, 7)
	var out, errB bytes.Buffer
	rc := runCycleReset([]string{"--project-root", root, "--dry-run"}, &out, &errB)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, errB.String())
	}
	if !strings.Contains(out.String(), "would seal cycle 7") {
		t.Errorf("stdout=%q, want dry-run verb for cycle 7 resolved via <root>/.evolve", out.String())
	}
}

func TestCycleResetCoverfixRootWarnerPrefixesAndTerminates(t *testing.T) {
	t.Parallel()
	var errB bytes.Buffer
	cycleResetRootWarner(&errB)("cannot resolve --project-root")
	if got, want := errB.String(), "evolve cycle reset: WARN: cannot resolve --project-root\n"; got != want {
		t.Errorf("stderr=%q, want %q", got, want)
	}
}

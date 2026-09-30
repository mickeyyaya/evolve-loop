package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComposedGatesTo_ReturnsPerGateTail(t *testing.T) {
	worktree := t.TempDir()
	makefile := "build:\n\t@true\n" +
		"test:\n\t@echo TAIL_MARKER_XYZ; exit 1\n" +
		"test-acs-durable:\n\t@true\n" +
		"apicover-enforce:\n\t@true\n"
	if err := os.MkdirAll(filepath.Join(worktree, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "go", "Makefile"), []byte(makefile), 0o644); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer

	results := composedGatesTo(&log)(context.Background(), worktree)

	failed, ok := results["test"]
	if !ok || failed.Status != "fail" {
		t.Fatalf("results[test] = %#v (ok=%v); want a fail outcome for the red gate", failed, ok)
	}
	if !strings.Contains(failed.Tail, "TAIL_MARKER_XYZ") {
		t.Errorf("results[test].Tail = %q; the gate's own output must ride in the returned outcome, not only the log sink", failed.Tail)
	}
	passed, ok := results["compile"]
	if !ok || passed.Status != "pass" {
		t.Errorf("results[compile] = %#v (ok=%v); a green gate must report pass with no tail", passed, ok)
	}
	if passed.Tail != "" {
		t.Errorf("results[compile].Tail = %q, want empty for a passing gate", passed.Tail)
	}
}

func TestComposedGatesTo_FailingGateOutputStaysOutOfTheLog(t *testing.T) {
	worktree := t.TempDir()
	makefile := "build:\n\t@true\n" +
		"test:\n\t@echo TEST_OUTPUT_ONE; echo TEST_OUTPUT_TWO; echo TEST_OUTPUT_THREE; exit 1\n" +
		"test-acs-durable:\n\t@echo ACS_OUTPUT_ONE; echo ACS_OUTPUT_TWO; exit 1\n" +
		"apicover-enforce:\n\t@true\n"
	if err := os.MkdirAll(filepath.Join(worktree, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "go", "Makefile"), []byte(makefile), 0o644); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer

	results := composedGatesTo(&log)(context.Background(), worktree)

	for gate, output := range map[string][]string{
		"test": {"TEST_OUTPUT_ONE", "TEST_OUTPUT_TWO", "TEST_OUTPUT_THREE"},
		"acs":  {"ACS_OUTPUT_ONE", "ACS_OUTPUT_TWO"},
	} {
		if results[gate].Status != "fail" {
			t.Errorf("results[%s] = %#v, want a fail outcome for the red gate", gate, results[gate])
		}
		for _, line := range output {
			if !strings.Contains(results[gate].Tail, line) {
				t.Errorf("results[%s].Tail = %q, must carry the gate's output line %q", gate, results[gate].Tail, line)
			}
			if strings.Contains(log.String(), line) {
				t.Errorf("log = %q dumps the %s gate's output line %q; the tail rides in the outcome only", log.String(), gate, line)
			}
		}
	}
	if lines := strings.Split(strings.TrimSpace(log.String()), "\n"); strings.TrimSpace(log.String()) != "" && len(lines) > 2 {
		t.Errorf("log = %q is a %d-line block; two failing gates may write at most one line each", log.String(), len(lines))
	}
}
